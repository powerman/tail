package tail_test

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"time"

	"github.com/powerman/tail"
)

func Example() {
	f, _ := os.CreateTemp("", "gotest")
	_, _ = f.Write([]byte("first"))

	ctx, cancel := context.WithCancel(context.Background())
	t := tail.Follow(ctx, tail.LoggerFunc(log.Printf), f.Name(),
		tail.PollTimeout(2*time.Second)) // increase timeout to avoid error from io.Copy

	go func() {
		time.Sleep(time.Second) // ensure tail has started

		_, _ = f.Write([]byte("second\n"))
		_ = os.Remove(f.Name())
		time.Sleep(time.Second) // ensure tail notice removed file

		_, _ = f.Write([]byte("third\n"))
		f.Close()
		time.Sleep(time.Second) // ensure tail read file before being cancelled

		cancel() // tell tail to cancel and return io.EOF
	}()

	_, err := io.Copy(os.Stdout, t)
	fmt.Println("err:", err)
	// Output:
	// second
	// third
	// err: <nil>
}
