package cli

import (
	"fmt"
	"io"
	"os"
	"time"
)

var spinnerFrames = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

// waitFor runs fn while showing message. A TTY gets a spinner; anything else
// gets a single "message..." line so tests and pipes stay quiet and parseable.
func waitFor(out io.Writer, message string, fn func() error) error {
	if !writerIsTerminal(out) {
		fmt.Fprintf(out, "%s...\n", message)
		return fn()
	}

	done := make(chan error, 1)
	go func() {
		done <- fn()
	}()

	ticker := time.NewTicker(80 * time.Millisecond)
	defer ticker.Stop()
	frame := 0
	writeSpinner(out, spinnerFrames[0], message)
	for {
		select {
		case err := <-done:
			if err != nil {
				writeSpinnerDone(out, "✗", message)
				return err
			}
			writeSpinnerDone(out, "✓", message)
			return nil
		case <-ticker.C:
			frame++
			writeSpinner(out, spinnerFrames[frame%len(spinnerFrames)], message)
		}
	}
}

func writeSpinner(out io.Writer, frame, message string) {
	fmt.Fprintf(out, "\r%s %s\033[K", frame, message)
}

func writeSpinnerDone(out io.Writer, mark, message string) {
	fmt.Fprintf(out, "\r%s %s\033[K\n", mark, message)
}

func writerIsTerminal(writer io.Writer) bool {
	file, ok := writer.(*os.File)
	if !ok {
		return false
	}
	return fileIsTerminal(file)
}

func readerIsTerminal(reader io.Reader) bool {
	file, ok := reader.(*os.File)
	if !ok {
		return false
	}
	return fileIsTerminal(file)
}

func fileIsTerminal(file *os.File) bool {
	info, err := file.Stat()
	return err == nil && info.Mode()&os.ModeCharDevice != 0
}
