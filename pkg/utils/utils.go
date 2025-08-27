package utils

import (
	"bufio"
	"os"
)

// EnsureFile 检查文件是否存在，不存在则创建，并写入内容；如果文件已经存在，则不做任何修改；
// 适合做初始化配置操作
func EnsureFile(filePath, content string) error {
	// 使用OpenFile以只写方式打开文件，如果文件不存在则创建它，
	// 如果文件已经存在，OpenFile不会清空文件，也不会写入新内容
	f, err := os.OpenFile(filePath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0644)
	if err != nil {
		// 如果出现错误，并且不是因为文件已经存在，则返回错误
		if !os.IsExist(err) {
			return err
		}
		// 文件已经存在，无需创建或写入，返回nil
		return nil
	}
	defer f.Close()

	// 向文件写入内容
	writer := bufio.NewWriter(f)

	if _, err := writer.WriteString(content); err != nil {
		return err
	}

	return writer.Flush()
}

// WriteFile: write text to file with name 'filename'
func WriteFile(filename string, text string) error {
	file, err := os.OpenFile(filename, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		return err
	}
	defer file.Close()

	writer := bufio.NewWriter(file)

	if _, err := writer.WriteString(text); err != nil {
		return err
	}

	return writer.Flush()
}
