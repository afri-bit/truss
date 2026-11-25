package runs

import (
	"fmt"
	"io"
)

// MaxRunLog bounds the log a daemon run keeps on disk (spec 028), so a step
// that prints without end cannot fill the disk. The run's status, steps and
// exit codes are kept in its record whatever the log holds.
const MaxRunLog = 64 << 20

// cappedLog writes until left bytes are used, then one line that says so,
// then nothing. Writes always report success, so the run goes on.
type cappedLog struct {
	w    io.Writer
	left int64
	cut  bool
}

func (c *cappedLog) Write(b []byte) (int, error) {
	if c.cut {
		return len(b), nil
	}
	if int64(len(b)) > c.left {
		_, _ = c.w.Write(b[:c.left])
		_, _ = fmt.Fprintf(c.w, "\n[the run log reached %d MiB; TRUSS keeps no more of it. The run goes on; see truss runs show for its result]\n", MaxRunLog>>20)
		c.cut = true
		return len(b), nil
	}
	c.left -= int64(len(b))
	return c.w.Write(b)
}
