package external

import (
  "log"
  "strings"
)

type Logger struct {}

func (l *Logger) Write(p []byte) (n int, err error) {
  buf := string(p)
  for _, line := range strings.Split(buf, "\n") {
    if l := strings.Trim(line, " \t"); l != "" {
      log.Print(string(l))
    }
  }
  return len(p), nil
}
