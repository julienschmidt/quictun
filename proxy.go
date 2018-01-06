package quictun

import (
	"io"
)

func proxy(dst io.WriteCloser, src io.Reader) {
	io.Copy(dst, src)
	//src.Close()
	dst.Close()
	//fmt.Println("done proxying")
}
