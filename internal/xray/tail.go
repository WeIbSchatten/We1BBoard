package xray

import (
	"bufio"
	"io"
	"os"
	"strings"
)

const tailMaxRead = 2 << 20 // 2 MiB from end

// TailFile returns the last n lines of path. Works on Windows (handles \r\n).
// Missing files return an empty slice without error.
func TailFile(path string, n int) ([]string, error) {
	if n <= 0 {
		return []string{}, nil
	}
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return []string{}, nil
		}
		return nil, err
	}
	defer f.Close()

	stat, err := f.Stat()
	if err != nil {
		return nil, err
	}
	size := stat.Size()
	if size == 0 {
		return []string{}, nil
	}

	var r io.Reader = f
	if size > tailMaxRead {
		if _, err := f.Seek(size-tailMaxRead, io.SeekStart); err != nil {
			return nil, err
		}
		// discard partial first line after seek
		br := bufio.NewReader(f)
		_, _ = br.ReadString('\n')
		r = br
	}

	data, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}
	text := strings.ReplaceAll(string(data), "\r\n", "\n")
	text = strings.ReplaceAll(text, "\r", "\n")
	raw := strings.Split(text, "\n")
	// drop trailing empty from final newline
	if len(raw) > 0 && raw[len(raw)-1] == "" {
		raw = raw[:len(raw)-1]
	}
	if len(raw) > n {
		raw = raw[len(raw)-n:]
	}
	return raw, nil
}
