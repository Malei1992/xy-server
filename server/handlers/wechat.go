package handlers

import (
	"bufio"
	"bytes"
	"context"
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

// sysProcAttrForKillGroup 返回让子进程独占进程组的 SysProcAttr(POSIX)。
// 配合 cmd.Cancel 用负号 PGID 杀整组,确保 `sh -c "sleep 60"` 里的 sleep 一并被 SIGKILL,
// 否则 exec.CommandContext 默认只杀 sh,sleep 变孤儿,cmd.Wait 要 60s 才返回。
func sysProcAttrForKillGroup() *syscall.SysProcAttr {
	return &syscall.SysProcAttr{Setpgid: true}
}

// qrLineRegex 判断单行"全是块字符 ▄/█ 或空白":用于识别 QR 块。
// 块字符之外的任何字符(数字、英文、标点)都让该行落选。
var qrLineRegex = regexp.MustCompile(`^[\s▄█]+$`)

// PostWechatBind 处理 POST /api/wechat/bind:
//  1. 顺序执行 commands(只取第一条,通常就是 docker exec 启动 openclaw 登录)
//  2. 实时读 stdout 到缓冲
//  3. 命令结束(或超时)后,解析出"最后一对" link 和 qr
//  4. 成功 → 200 + {ok, link, qr, expired:false, raw};失败 → 500 + {error, output, expired}
//
// 契约:
//   - docker 启动失败(非超时) → expired=false
//   - 启动成功但 timeout 内没结束 → expired=true
//   - 拿到 link 但没 qr(反之亦然) → 200,缺的字段为空串
//
// commands 设计成可注入:生产传 [[docker exec ...]],测试传 [[sh -c "..."]]。
func PostWechatBind(commands [][]string, timeout time.Duration) gin.HandlerFunc {
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
		L.Info("wechat bind: starting", zap.Strings("cmd", args), zap.Duration("timeout", timeout))

		ctx, cancel := context.WithTimeout(c.Request.Context(), timeout)
		defer cancel()

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
			L.Error("wechat bind: stdout pipe failed", zap.Error(err))
			c.JSON(http.StatusInternalServerError, gin.H{
				"error":   "wechat bind failed: " + err.Error(),
				"output":  "",
				"expired": false,
			})
			return
		}
		// docker exec 偶尔有 stderr 噪音,一并吞到缓冲
		var stderrBuf bytes.Buffer
		cmd.Stderr = &stderrBuf

		if err := cmd.Start(); err != nil {
			L.Error("wechat bind: command start failed", zap.Strings("cmd", args), zap.Error(err))
			c.JSON(http.StatusInternalServerError, gin.H{
				"error":   "wechat bind failed: " + err.Error(),
				"output":  "",
				"expired": false,
			})
			return
		}

		// 实时读取 stdout(避免 30s 后才知道输出)
		var stdoutBuf bytes.Buffer
		scanner := bufio.NewScanner(stdout)
		// QR 单行可达 32 字符(块字符) + 控制序列;给足 buffer 防止截断
		scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
		for scanner.Scan() {
			stdoutBuf.Write(scanner.Bytes())
			stdoutBuf.WriteByte('\n')
		}
		// scanner.Err() 在 wait 之后再看也无妨,wait 才是权威退出原因
		waitErr := cmd.Wait()

		// ctx 超时 → cmd.Wait() 报 context deadline exceeded,且子进程已被 kill
		expired := waitErr != nil && ctx.Err() == context.DeadlineExceeded

		raw := stdoutBuf.String()
		// 失败时把 stderr 拼到 output 末尾,排障用
		if stderrBuf.Len() > 0 {
			raw += "\n[stderr]\n" + stderrBuf.String()
		}

		link, qr := parseWechatOutput(raw)

		// 失败情况细分:
		//   - 超时(ctx deadline exceeded) → expired=true
		//   - 启动后 exit 非 0(容器不存在、openclaw 报错) → expired=false
		if waitErr != nil {
			if expired {
				L.Warn("wechat bind: timeout",
					zap.Duration("timeout", timeout),
					zap.Int("stdout_size", stdoutBuf.Len()),
				)
			} else {
				L.Error("wechat bind: command failed",
					zap.Strings("cmd", args),
					zap.Error(waitErr),
				)
			}
			c.JSON(http.StatusInternalServerError, gin.H{
				"error":   "wechat bind failed: " + waitErr.Error(),
				"output":  raw,
				"expired": expired,
			})
			return
		}

		// 命令成功(exit 0)但 link 和 qr 都没拿到:openclaw 没正常输出,按失败处理
		if link == "" && qr == "" {
			L.Warn("wechat bind: no link/qr found in stdout", zap.Int("stdout_size", stdoutBuf.Len()))
			c.JSON(http.StatusInternalServerError, gin.H{
				"error":   "wechat bind failed: no link or qr in output",
				"output":  raw,
				"expired": false,
			})
			return
		}

		// 拿到 link 或 qr(允许另一个缺失) → 200,缺的字段空串
		L.Info("wechat bind: ok",
			zap.String("link", link),
			zap.Int("qr_lines", strings.Count(qr, "\n")+1),
			zap.Int("raw_size", len(raw)),
		)
		c.JSON(http.StatusOK, gin.H{
			"ok":      true,
			"link":    link,
			"qr":      qr,
			"expired": false,
			"raw":     raw,
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
		start  int
		lines  []string
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
