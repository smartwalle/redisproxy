package protocol

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"strconv"
)

// 简单的 RESP（Redis Serialization Protocol）编解码。
// 仅用于认证阶段解析 AUTH 命令，以及后端连接握手阶段构造
// AUTH / SELECT 命令。认证完成后的数据面走透明转发，不再解析 RESP。

// WriteCommand 以 RESP 数组格式编码一条命令并写入 w。
// 例如 WriteCommand(w, "AUTH", "secret") 编码为
// "*2\r\n$4\r\nAUTH\r\n$6\r\nsecret\r\n"。
func WriteCommand(w io.Writer, args ...string) error {
	var buf bytes.Buffer
	buf.WriteByte('*')
	buf.WriteString(strconv.Itoa(len(args)))
	buf.WriteString("\r\n")
	for _, a := range args {
		buf.WriteByte('$')
		buf.WriteString(strconv.Itoa(len(a)))
		buf.WriteString("\r\n")
		buf.WriteString(a)
		buf.WriteString("\r\n")
	}
	if _, err := buf.WriteTo(w); err != nil {
		return err
	}
	return nil
}

// ReadCommand 从 br 中解析一条 RESP 数组命令，返回各参数。
func ReadCommand(br *bufio.Reader) ([]string, error) {
	line, err := readLine(br)
	if err != nil {
		return nil, err
	}
	if len(line) == 0 || line[0] != '*' {
		return nil, fmt.Errorf("unsupported request: %q", string(line))
	}

	n, err := strconv.Atoi(string(line[1:]))
	if err != nil {
		return nil, fmt.Errorf("invalid multibulk length: %w", err)
	}
	if n <= 0 || n > 1024*1024 {
		return nil, fmt.Errorf("invalid multibulk length: %d", n)
	}

	args := make([]string, 0, n)
	for i := 0; i < n; i++ {
		line, err = readLine(br)
		if err != nil {
			return nil, err
		}
		if len(line) == 0 || line[0] != '$' {
			return nil, fmt.Errorf("expected '$', got %q", string(line))
		}
		size, err := strconv.Atoi(string(line[1:]))
		if err != nil {
			return nil, fmt.Errorf("invalid bulk length: %w", err)
		}
		if size < 0 || size > 512*1024*1024 {
			return nil, fmt.Errorf("invalid bulk length: %d", size)
		}
		buf := make([]byte, size+2)
		if _, err := io.ReadFull(br, buf); err != nil {
			return nil, err
		}
		args = append(args, string(buf[:size]))
	}

	return args, nil
}

// ReadStatus 读取并校验 RESP 简单字符串回复，期望以 +OK 开头。
func ReadStatus(conn io.Reader) error {
	br := bufio.NewReader(conn)
	line, err := br.ReadString('\n')
	if err != nil {
		return err
	}
	if len(line) < 2 || line[len(line)-2] != '\r' {
		return errors.New("invalid line ending")
	}
	reply := line[:len(line)-2]
	if len(reply) == 0 {
		return errors.New("empty reply")
	}
	switch reply[0] {
	case '+':
		return nil
	case '-':
		return fmt.Errorf("redis error: %s", reply[1:])
	default:
		return fmt.Errorf("unexpected reply: %s", reply)
	}
}

// readLine 读取以 \r\n 结尾的一行，返回去掉结尾符后的内容。
func readLine(br *bufio.Reader) ([]byte, error) {
	line, err := br.ReadBytes('\n')
	if err != nil {
		return nil, err
	}
	if len(line) < 2 || line[len(line)-2] != '\r' {
		return nil, errors.New("invalid line ending")
	}
	return line[:len(line)-2], nil
}
