// +build cgo

package jpegsimp

/*
#cgo linux CFLAGS: -I/usr/include
#cgo linux LDFLAGS: -ljpeg -L/usr/lib
#cgo darwin CFLAGS: -I/opt/libjpeg-turbo/include -I/opt/local/include -DIM_DEBUG
#cgo darwin LDFLAGS: -ljpeg -L/opt/libjpeg-turbo/lib -L/opt/local/lib

// debug CFLAGS:
// -DIM_DEBUG for simp_image output
// -DJPEG_DEBUG for jpeg output, depend IM_DEBUG

#include "c-jpeg.h"

static unsigned char ** makeCharArray(int size) {
    return calloc(sizeof(unsigned char*), size);
}

//static void setArrayString(char **a, char *s, int n) {
//    a[n] = s;
//}

//static void freeCharArray(char **a, int size) {
//    int i;
//    for (i = 0; i < size; i++)
//        free(a[i]);
//    free(a);
//}

*/
import "C"

import (
	"errors"
	"io"
	"io/ioutil"
	"log"
	"os"
	"unsafe"
)

// consts
const (
	MinQuality = 75
)

//export log_print
func log_print(cs *C.char) {
	GetLogger().Printf(">\t%s\n", C.GoString(cs))
}

// Simper JPEG simple object
type Simper interface {
	GetAttr() *Attr
	SetOption(wopt WriteOption)
	WriteTo(out io.Writer) error
	GetBlob() ([]byte, error)
	Close() error
}

// jpeg simp_image
type simpJPEG struct {
	si   *C.Simp_Image
	wopt WriteOption
	// size Size
	*Attr
}

func newSimpJPEG() *simpJPEG {
	o := &simpJPEG{}
	return o
}

// Open ...
func Open(r io.Reader) (m Simper, err error) {
	var si *C.Simp_Image
	if f, ok := r.(*os.File); ok {
		log.Println("open file reader")
		f.Seek(0, 0)
		cmode := C.CString("rb")
		defer C.free(unsafe.Pointer(cmode))
		infile := C.fdopen(C.int(f.Fd()), cmode)
		//defer C.fclose(infile)
		si = C.simp_open_file(infile)
		if si == nil {
			err = errors.New("simp_open_file failed")
			return
		}
	} else {
		var blob []byte
		blob, err = ioutil.ReadAll(r)

		if err != nil {
			GetLogger().Printf("read err %s", err)
			return
		}

		ln := len(blob)
		GetLogger().Printf("jpeg blob (%d) head: %x, tail: %x", ln, blob[0:8], blob[ln-2:ln])

		GetLogger().Printf("open mem buf len %d\n", ln)
		p := (*C.uchar)(unsafe.Pointer(&blob[0]))

		si = C.simp_open_mem(p, C.uint(ln))
		if si == nil {
			err = errors.New("simp_open_mem failed")
			return
		}
	}

	w := C.simp_get_width(si)
	h := C.simp_get_height(si)
	q := C.simp_get_quality(si)
	GetLogger().Printf("image open, w: %d, h: %d, q: %d", w, h, q)

	attr := NewAttr(uint(w), uint(h), uint8(q))
	m = &simpJPEG{si: si, Attr: attr}
	return
}

// GetAttr ...
func (sj *simpJPEG) GetAttr() *Attr {
	return sj.Attr
}

// Close implemnets for io.Closer
func (sj *simpJPEG) Close() error {
	if sj.si != nil {
		C.simp_close(sj.si)
		sj.si = nil
	}
	return nil
}

func (sj *simpJPEG) SetOption(wopt WriteOption) {
	GetLogger().Printf("setOption: q %d, s %v", wopt.Quality, wopt.StripAll)
	if wopt.Quality < MinQuality {
		sj.wopt.Quality = MinQuality
	} else {
		sj.wopt.Quality = wopt.Quality
	}
	C.simp_set_quality(sj.si, C.int(sj.wopt.Quality))
	GetLogger().Printf("set quality: %d", sj.wopt.Quality)

	sj.wopt.StripAll = wopt.StripAll
}

// WriteTo ...
func (sj *simpJPEG) WriteTo(out io.Writer) error {
	if f, ok := out.(*os.File); ok {
		GetLogger().Printf("write a file %s\n", f.Name())

		ocmode := C.CString("wb")
		defer C.free(unsafe.Pointer(ocmode))
		outfile := C.fdopen(C.int(f.Fd()), ocmode)

		r := C.simp_output_file(sj.si, outfile)
		if !r {
			log.Println("simp out file error")
			return errors.New("output error")
		}

	} else {
		log.Println("write to buf")

		data, err := sj.GetBlob()
		if err != nil {
			return err
		}
		// GetLogger().Printf("blob %d bytes\n", len(data))

		ret, err := out.Write(data)
		if err != nil {
			// log.Println(err)
			return err
		}

		GetLogger().Printf("writed %d\n", ret)
	}

	return nil
}

// GetBlob ...
func (sj *simpJPEG) GetBlob() ([]byte, error) {
	cblob := (**C.uchar)(C.makeCharArray(C.int(0)))
	*cblob = nil
	defer C.free(unsafe.Pointer(cblob))

	var size = 0
	r := C.simp_output_mem(sj.si, cblob, (*C.ulong)(unsafe.Pointer(&size)))

	if !r || *cblob == nil {
		log.Println("simp out mem error")
		// return errors.New("output error")
		return nil, errors.New("output error")
	}
	GetLogger().Printf("c output %d bytes\n", size)
	return C.GoBytes(unsafe.Pointer(*cblob), C.int(size)), nil
}

// Optimize ...
func Optimize(r io.Reader, w io.Writer, wopt *WriteOption) (sizeIn, sizeOut int, err error) {

	var cr = new(CountWriter)
	var im Simper
	im, err = Open(io.TeeReader(r, cr))

	if err != nil {
		// log.Println(err)
		return
	}
	defer im.Close()
	im.SetOption(*wopt)

	var cw = new(CountWriter)
	err = im.WriteTo(io.MultiWriter(w, cw))
	if err != nil {
		// log.Println(err)
		return
	}

	sizeIn = cr.Len()
	sizeOut = cw.Len()
	ratio := float64(sizeIn-sizeOut) * 100.0 / float64(sizeIn)
	GetLogger().Printf("%d --> %d bytes (%0.2f%%), optimized.\n", sizeIn, sizeOut, ratio)

	return
}
