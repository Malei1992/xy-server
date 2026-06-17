package handlers

import (
	"errors"
	"os"
)

// readJSONFile 读文件，不存在时返回明确错误，方便上层区分 404 / 500。
func readJSONFile(path string) ([]byte, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, errors.New("file not found")
		}
		return nil, err
	}
	return data, nil
}