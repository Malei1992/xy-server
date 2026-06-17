package handlers

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"net/http"
	"os/exec"
	"regexp"
	"strings"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// QR 块最小行数:docker 输出常见为 20 行;少于 5 行视为噪声,忽略。
// 防止"单行 ▄/█ 误识别"和"散落的 ▄ 字符触发假阳性"。
const qrBlockMinLines = 5

// wechatLinkRegex 匹配微信扫码链接。一个非空白字符结尾,足以涵盖 query string。
var wechatLinkRegex = regexp.MustCompile(`https://liteapp\.weixin\.qq\.com/q/\S+`)

// wechatBoundRegex 匹配 openclaw 成功连接微信后的输出:
//   「已将此 OpenClaw 连接到微信。」
// 看到这一行说明用户已经扫码并确认,exec 即将自然退出。
// 用于在 scanner 循环里早期设 Bound=true,前端 polling 拿到 bound:true 后
// 切到「绑定成功」状态,不等 cmd.Wait()。
var wechatBoundRegex = regexp.MustCompile(`已将此 OpenClaw 连接到微信`)

// wechatAlreadyBoundRegex 匹配 openclaw 提示"该 OpenClaw 之前已经连过微信,
// 不用重复连"的输出:
//   「已连接过此 OpenClaw，无需重复连接」
// 含义:用户点绑定时 openclaw 检测到这个 gateway 已经绑过某个微信账号,
// 所以没有真正发起新连接。exec 会自然退出(exit 0)且 status=done。
//
// 处理:
//   - 设 Bound=true(openclaw 这次没失败,只是 noop,用户视角"已经绑定" = 成功)
//   - 设 AlreadyBound=true 让前端区分提示
//   - 跳过 scheduleOpenclawConfigSync(openclaw.json 早就同步过了,bot IDs 早已登记,
//     重复 sync 是无操作但仍会创建空 goroutine;不调用更省)
var wechatAlreadyBoundRegex = regexp.MustCompile(`已连接过此 OpenClaw，无需重复连接`)

// sysProcAttrForKillGroup 返回让子进程独占进程组的 SysProcAttr(POSIX)。
// 配合 cmd.Cancel 用负号 PGID 杀整组,确保 `sh -c "sleep 60"` 里的 sleep 一并被 SIGKILL,
// 否则 exec.CommandContext 默认只杀 sh,sleep 变孤儿,cmd.Wait 要 60s 才返回。
func sysProcAttrForKillGroup() *syscall.SysProcAttr {
	return &syscall.SysProcAttr{Setpgid: true}
}

// qrLineRegex 判断单行"全是 Unicode 块字符或空白":用于识别 QR 块。
// 块字符之外的任何字符(数字、英文、标点)都让该行落选。
//
// 范围:Unicode Block 元素 U+2580–U+259F(▀▁▂▃▄▅▆▇█▉▊▋▌▍▎▏)
//  + 块元字符 U+2580(上块) U+2584(下块 ▄) U+2588(全块 █) 等
//  + 阴影块 U+2591–U+2593(░▒▓)
// openclaw / 各类终端二维码库都会用这一组(不只 ▄/█ 两个),
// 之前只放 ▄/█ 会让每行匹配都失败 → QR 块被切成 1 行小段 → < minLines → 解析空。
var qrLineRegex = regexp.MustCompile(`^[\s▀▁▂▃▄▅▆▇█▉▊▋▌▍▎▏░▒▓]+$`)

// PostWechatBind 处理 POST /api/wechat/bind: **非阻塞** + 轮询模式。
//
//  1. 立即建一个 pending task,返回 202 + task_id 给前端(几十毫秒返回,不阻塞 gin 协程)
//  2. 起后台 goroutine: 状态 → running → 执行 docker exec(沿用 parseWechatOutput 解析)
//     → 终态 done/failed/expired → 记 CompletedAt 给 TTL 清理
//  3. 前端拿 task_id 轮询 GET /api/wechat/bind/:task_id 拿结果
//
// 契约:
//   - docker 启动失败(非超时) → status=failed, error 含 docker 错误, expired=false
//   - 启动成功但 timeout 内没结束 → status=expired, expired=true
//   - 拿到 link 但没 qr(反之亦然) → status=done, 缺的字段空串
//
// commands 设计成可注入:生产传 [[docker exec ...]],测试传 [[sh -c "..."]]。
// timeout 传给后台 goroutine,handler 本身不等。
// baseDir 是 openclaw.json / openclaw-weixin/accounts.json 所在目录;用于 sync 新 bot IDs。
func PostWechatBind(commands [][]string, timeout time.Duration, baseDir string) gin.HandlerFunc {
	return func(c *gin.Context) {
		if len(commands) == 0 || len(commands[0]) == 0 {
			L.Error("wechat bind: no commands configured")
			c.JSON(http.StatusInternalServerError, gin.H{
				"error":   "wechat bind failed: no commands configured",
				"output":  "",
				"expired": false,
			})
			return
		}
		args := commands[0]

		// 1. 立即建 task 并返回 202;后台 goroutine 跑 docker。
		taskID, task := GetWechatTaskStore().Add()
		task.Status = StatusPending
		L.Info("wechat bind: task created",
			zap.String("task_id", taskID),
			zap.Strings("cmd", args),
			zap.Duration("timeout", timeout),
		)

		// 2. 后台 goroutine:pending → running → 终态
		go runWechatBindTask(taskID, args, timeout, baseDir)

		c.JSON(http.StatusAccepted, gin.H{
			"task_id": taskID,
			"status":  string(StatusPending),
		})
	}
}

// runWechatBindTask 是后台 goroutine 入口。推进一个 task 的完整生命周期:
//   - running
//   - 执行 docker exec,流式读 stdout
//   - 根据 waitErr 判断 done / failed / expired
//   - 终态设 CompletedAt,TTL 清理会按这个时间算
//
// 复用原同步逻辑(parseWechatOutput、Setpgid、Cancel 杀进程组)以保证行为一致。
func runWechatBindTask(taskID string, args []string, timeout time.Duration, baseDir string) {
	store := GetWechatTaskStore()

	store.Update(taskID, func(t *WechatTask) {
		t.Status = StatusRunning
	})
	L.Info("wechat bind: task running", zap.String("task_id", taskID))

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	// 暴露给 POST /api/wechat/bind/:task_id/cancel 用:
	// 用户关掉 modal 时调 cancel(),通过 context 触发 cmd.Cancel 杀整个进程组
	// (kill -SIGKILL -PGID),不用等 2 分钟 timeout。
	store.Update(taskID, func(t *WechatTask) {
		t.cancel = cancel
	})

	cmd := exec.CommandContext(ctx, args[0], args[1:]...)
	// 子进程放新进程组(Setpgid=true),超时 SIGKILL 时连同 sh -c "sleep 60" 里的 sleep 一起杀。
	// 不设的话 CommandContext 只杀 sh,sleep 变孤儿,cmd.Wait 要 60s 才返回。
	cmd.SysProcAttr = sysProcAttrForKillGroup()
	// Cancel 在 context 超时/取消时被 Go runtime 调,默认只 SIGKILL 当前进程。
	// 这里改成杀整个进程组(负号 PGID),确保所有子进程一起挂。
	cmd.Cancel = func() error {
		if cmd.Process == nil {
			return nil
		}
		// PGID = 子进程自身 PID(Setpgid 把它设成新组 leader)
		return syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		L.Error("wechat bind: stdout pipe failed",
			zap.String("task_id", taskID),
			zap.Error(err),
		)
		finishWechatTaskFailed(store, taskID, "stdout pipe: "+err.Error(), "", false)
		return
	}
	// docker exec 偶尔有 stderr 噪音,一并吞到缓冲
	var stderrBuf bytes.Buffer
	cmd.Stderr = &stderrBuf

	if err := cmd.Start(); err != nil {
		L.Error("wechat bind: command start failed",
			zap.String("task_id", taskID),
			zap.Strings("cmd", args),
			zap.Error(err),
		)
		finishWechatTaskFailed(store, taskID, err.Error(), "", false)
		return
	}

	// 实时读取 stdout(避免 30s 后才知道输出)
	var stdoutBuf bytes.Buffer
	scanner := bufio.NewScanner(stdout)
	// QR 单行可达 32 字符(块字符) + 控制序列;给足 buffer 防止截断
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	// 早期发布标志:openclaw 实际输出顺序是 QR 块先,link 行后(且 link 行唯一)。
	// 看到 link 那一行就说明 QR 已经在 stdout 里了 —— 此时把 link/qr 写进 store,
	// 前端 polling 下个 tick 就能拿到,不用死等 2 分钟 SIGKILL 才把 link/qr 写进去。
	// 不提前到 QR 块那一行是因为:
	//   1) QR 块通常 20+ 行,在它还没打完时 parseWechatOutput 已经能拿到部分,但完整版要等
	//   2) link 是单行单次匹配,信号清晰,且语义自洽(看见 link = 流程已走到「可以扫」阶段)
	// 一次性发布:`dataPublished` 保证不重复 parse/store.Update。
	//
	// bound 标志独立:用户扫码后 stdout 出现「已将此 OpenClaw 连接到微信。」,
	// 不代表 link/qr 已经抓完(虽然按 openclaw 实际输出顺序 link/qr 必在它之前),
	// 但概念上是独立事件 —— link/qr 是「可以扫了」,bound 是「扫成功了」。
	// 用独立标志避免互相覆盖。
	dataPublished := false
	boundPublished := false
	for scanner.Scan() {
		stdoutBuf.Write(scanner.Bytes())
		stdoutBuf.WriteByte('\n')

		if !dataPublished && wechatLinkRegex.Match(scanner.Bytes()) {
			partialRaw := stdoutBuf.String()
			partialLink, partialQR := parseWechatOutput(partialRaw)
			if partialLink != "" && partialQR != "" {
				dataPublished = true
				L.Info("wechat bind: early publish link+qr (waiting for cmd.Wait final state)",
					zap.String("task_id", taskID),
					zap.String("link", partialLink),
					zap.Int("qr_lines", strings.Count(partialQR, "\n")+1),
				)
				store.Update(taskID, func(t *WechatTask) {
					// 防御性:不覆盖已存在的 link/qr(虽然理论上没人写过,但写防御不为过)
					if t.Link == "" && t.QR == "" {
						t.Link = partialLink
						t.QR = partialQR
					}
				})
			}
		}

		// 成功标记早期发布:用户扫码后 openclaw 立刻输出这一行。
		// 不立即设 Bound=true——先等 2s + sync openclaw 配置(把新 bot IDs
		// 写到 openclaw.json),然后再 Bound=true。前端看到 Bound=true 时
		// 配置已经同步完,可以安全关 modal。
		//
		// 为什么 Bound 不能立刻置 true:
		//   - 用户原话:"这个步骤操作完后,在返回给前端,绑定完成"
		//   - Bound=true 前置会让前端立刻关 modal,但 sync 失败时配置没同步,
		//     下次 wechat agent 启动会找不到 binding,排查难
		if !boundPublished && wechatAlreadyBoundRegex.Match(scanner.Bytes()) {
			boundPublished = true
			L.Info("wechat bind: detected already-bound marker (noop)",
				zap.String("task_id", taskID),
			)
			store.Update(taskID, func(t *WechatTask) {
				t.AlreadyBound = true
				t.Bound = true
			})
			// 注意:不调 scheduleOpenclawConfigSync。已绑定场景下 openclaw.json
			// 早就同步过了,bot IDs 早已登记,重复 sync 是无操作但仍会创建空 goroutine。
		}
		if !boundPublished && wechatBoundRegex.Match(scanner.Bytes()) {
			boundPublished = true
			L.Info("wechat bind: detected success marker, scheduling config sync",
				zap.String("task_id", taskID),
				zap.Duration("sync_delay", openclawConfigSyncDelay),
			)
			go scheduleOpenclawConfigSync(taskID, baseDir)
		}
	}
	// scanner.Err() 在 wait 之后再看也无妨,wait 才是权威退出原因
	waitErr := cmd.Wait()

	// ctx 取消(超时 OR 用户主动 cancel)→ cmd.Wait() 报非 nil,且子进程已被 kill。
	// 这里 ctx.Err() != nil 涵盖两种情况:context.DeadlineExceeded(timeout)和
	// context.Canceled(用户调 POST /cancel 端点)。前端对此统一视为"进程被中断",
	// 区别只在 error 字段文案。
	expired := waitErr != nil && ctx.Err() != nil
	userCancelled := waitErr != nil && errors.Is(ctx.Err(), context.Canceled)

	raw := stdoutBuf.String()
	// 失败时把 stderr 拼到 output 末尾,排障用
	if stderrBuf.Len() > 0 {
		raw += "\n[stderr]\n" + stderrBuf.String()
	}

	// 不论是 done / failed / expired 都先解析一次 link/qr。
	// 原因:超时时 openclaw 通常已经打印了 QR + 链接(只是不会自动退出),
	// 此时虽然 waitErr != nil,前端仍要把 QR 给用户扫 —— 进程被中断但 QR 可能还有效。
	// expired/failed 路径下 link/qr 仍非空时,Status 走 expired/failed,
	// 但 Link/QR 字段照填,前端 modal 顶部加"可能有效"提示让用户试扫。
	link, qr := parseWechatOutput(raw)

	// 失败情况细分:
	//   - ctx 取消(超时 / 用户 cancel)→ expired=true, status=expired
	//   - 启动后 exit 非 0(容器不存在、openclaw 报错) → expired=false, status=failed
	if waitErr != nil {
		if expired {
			reason := "timeout"
			if userCancelled {
				reason = "user cancelled"
			}
			L.Warn("wechat bind: ctx cancelled",
				zap.String("task_id", taskID),
				zap.String("reason", reason),
				zap.Duration("timeout", timeout),
				zap.Int("stdout_size", stdoutBuf.Len()),
				zap.Bool("has_link", link != ""),
				zap.Bool("has_qr", qr != ""),
			)
			store.Update(taskID, func(t *WechatTask) {
				t.Status = StatusExpired
				t.Expired = true
				// 用户主动 cancel:用专属文案,前端可显示「已取消」而不是「超时」
				if userCancelled {
					t.Error = "wechat bind cancelled by user"
				} else {
					t.Error = "wechat bind failed: " + waitErr.Error()
				}
				t.Link = link
				t.QR = qr
				t.Raw = raw
				t.CompletedAt = time.Now()
			})
		} else {
			L.Error("wechat bind: command failed",
				zap.String("task_id", taskID),
				zap.Strings("cmd", args),
				zap.Error(waitErr),
				zap.Bool("has_link", link != ""),
				zap.Bool("has_qr", qr != ""),
			)
			store.Update(taskID, func(t *WechatTask) {
				t.Status = StatusFailed
				t.Expired = false
				t.Error = "wechat bind failed: " + waitErr.Error()
				t.Link = link
				t.QR = qr
				t.Raw = raw
				t.CompletedAt = time.Now()
			})
		}
		return
	}

	// 命令成功(exit 0)但 link 和 qr 都没拿到:openclaw 没正常输出,按失败处理
	if link == "" && qr == "" {
		L.Warn("wechat bind: no link/qr found in stdout",
			zap.String("task_id", taskID),
			zap.Int("stdout_size", stdoutBuf.Len()),
		)
		store.Update(taskID, func(t *WechatTask) {
			t.Status = StatusFailed
			t.Expired = false
			t.Error = "wechat bind failed: no link or qr in output"
			t.Link = link
			t.QR = qr
			t.Raw = raw
			t.CompletedAt = time.Now()
		})
		return
	}

	// 拿到 link 或 qr(允许另一个缺失) → status=done
	L.Info("wechat bind: ok",
		zap.String("task_id", taskID),
		zap.String("link", link),
		zap.Int("qr_lines", strings.Count(qr, "\n")+1),
		zap.Int("raw_size", len(raw)),
	)
	store.Update(taskID, func(t *WechatTask) {
		t.Status = StatusDone
		t.Link = link
		t.QR = qr
		t.Raw = raw
		t.Expired = false
		t.CompletedAt = time.Now()
	})
}

// finishWechatTaskFailed 简化版:在还没起 cmd.Start 之前就挂掉时的统一出口。
func finishWechatTaskFailed(store *WechatTaskStore, taskID, errMsg, raw string, expired bool) {
	store.Update(taskID, func(t *WechatTask) {
		t.Status = StatusFailed
		t.Expired = expired
		t.Error = "wechat bind failed: " + errMsg
		t.Raw = raw
		t.CompletedAt = time.Now()
	})
}

// GetWechatBindStatus 处理 GET /api/wechat/bind/:task_id,前端轮询拿 task 状态。
//   - task_id 不存在或被 TTL 清理 → 404 {"error":"task not found"}
//   - 找到 → 200,字段为当前 task 快照(task_id, status, link, qr, raw, expired, error)
//
// 必须在 store 层做一次值拷贝,而不是直接拿 *WechatTask 引用读字段——
// 后台 goroutine 会并发写 link/qr/raw/error,直接读会 data race。
func GetWechatBindStatus() gin.HandlerFunc {
	return func(c *gin.Context) {
		taskID := c.Param("task_id")
		t, ok := GetWechatTaskStore().GetSnapshot(taskID)
		if !ok {
			c.JSON(http.StatusNotFound, gin.H{"error": "task not found"})
			return
		}
		c.JSON(http.StatusOK, gin.H{
			"task_id":       t.TaskID,
			"status":        string(t.Status),
			"link":          t.Link,
			"qr":            t.QR,
			"raw":           t.Raw,
			"expired":       t.Expired,
			"bound":         t.Bound,
			"error":         t.Error,
			"sync_error":    t.SyncError,
			"already_bound": t.AlreadyBound,
		})
	}
}

// PostWechatBindCancel 处理 POST /api/wechat/bind/:task_id/cancel,让前端在用户
// 关闭 modal 时主动结束 exec(免等 2 分钟 timeout)。
//
// 语义:
//   - task_id 不存在 / 已 TTL 清理 → 404 {"error":"task not found"}
//   - task 已是终态(done/failed/expired) → 200,error="task already finished",不重复 kill
//   - task 还在 running → 调 cancel() 触发 cmd.Cancel 杀整个进程组,设 expired=true,
//     写 error="user cancelled"
//
// cancel() 走 context cancel,触发 cmd.Cancel(SIGKILL -PGID)同步收尾;
// runWechatBindTask 里的 cmd.Wait() 在 cancel 后会返回,后续流程跟 timeout 路径一致。
func PostWechatBindCancel() gin.HandlerFunc {
	return func(c *gin.Context) {
		taskID := c.Param("task_id")
		store := GetWechatTaskStore()
		t, ok := store.GetSnapshot(taskID)
		if !ok {
			c.JSON(http.StatusNotFound, gin.H{"error": "task not found"})
			return
		}
		// 已终态:不重复 cancel,直接返 200。cancel 字段没暴露给 JSON 走 Get
		// 拿到的是 snapshot,直接读 cancel 没意义,需要再 Get 原引用。
		switch t.Status {
		case StatusDone, StatusFailed, StatusExpired:
			c.JSON(http.StatusOK, gin.H{
				"task_id": taskID,
				"status":  string(t.Status),
				"cancelled": false,
				"reason": "task already finished",
			})
			return
		}
		// 拿 store 里的原引用,调 cancel
		orig, ok := store.Get(taskID)
		if !ok {
			c.JSON(http.StatusNotFound, gin.H{"error": "task not found"})
			return
		}
		var cancelFn context.CancelFunc
		orig.locker.mu.Lock()
		cancelFn = orig.cancel
		orig.locker.mu.Unlock()
		if cancelFn == nil {
			// cancel 还没初始化(背景 goroutine 还没跑到 store.Update(cancel))
			// 这种场景极少,返 503 让前端等几百 ms 重试
			c.JSON(http.StatusServiceUnavailable, gin.H{
				"error":   "cancel not ready yet",
				"task_id": taskID,
			})
			return
		}
		cancelFn()
		L.Info("wechat bind: user cancel requested",
			zap.String("task_id", taskID),
		)
		c.JSON(http.StatusOK, gin.H{
			"task_id":    taskID,
			"cancelled":  true,
		})
	}
}

// parseWechatOutput 从 docker openclaw 登录输出中提取"最后一对"链接和 QR 块。
// 链接:所有 liteapp.weixin.qq.com 链接取最后一个。
// QR 块:连续 >= qrBlockMinLines 行,每行只含 ▄/█/空白,取最后一个这样的连续块。
// 没找到返回空串(由调用方决定"成功但缺字段"还是"失败")。
func parseWechatOutput(raw string) (link, qr string) {
	// --- 1. 提取链接(最后一个) ---
	for _, m := range wechatLinkRegex.FindAllString(raw, -1) {
		link = m
	}

	// --- 2. 提取 QR 块(最后一个) ---
	// 按行扫描,把连续的"块字符行"合成块;挑长度 >= minLines 的,记下最后一个。
	scanner := bufio.NewScanner(strings.NewReader(raw))
	// QR 单行可能含 ANSI 控制序列或较长块字符串,放大 buffer
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	var curLines []string
	curStart := -1
	type block struct {
		start int
		lines []string
	}
	var blocks []block
	lineNo := 0
	flush := func() {
		if len(curLines) >= qrBlockMinLines {
			blocks = append(blocks, block{start: curStart, lines: append([]string(nil), curLines...)})
		}
		curLines = nil
		curStart = -1
	}
	for scanner.Scan() {
		line := scanner.Text()
		if qrLineRegex.MatchString(line) && strings.ContainsAny(line, "▄█") {
			if curStart < 0 {
				curStart = lineNo
			}
			curLines = append(curLines, line)
		} else {
			flush()
		}
		lineNo++
	}
	flush()

	if len(blocks) > 0 {
		last := blocks[len(blocks)-1]
		qr = strings.Join(last.lines, "\n")
	}
	return link, qr
}
