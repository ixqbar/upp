package upp

import (
	"crypto/md5"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"path"
	"strings"
)

func CheckFileIsDirectory(path string) (bool, error)  {
	fi, err := os.Stat(path)
	if err != nil {
		return false, err
	}

	if fi.IsDir() == false {
		return false, errors.New("target file is not folder")
	}

	return true, nil
}

func GetFileSize(file string) (int64, error) {
	fi, err := os.Stat(file)
	if err != nil {
		return 0, err
	}

	if fi.IsDir() {
		return 0, errors.New(fmt.Sprintf("target file %s is not file", file))
	}

	return fi.Size(), nil
}

func InStringArray(value string, arrays []string) bool {
	for _, v := range arrays {
		if v == value {
			return true
		}
	}

	return false
}

func GetFileMD5sum(file string) (string, error) {
	f, err := os.Open(file)
	if err != nil {
		return "", err
	}

	defer f.Close()

	h := md5.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}

	return hex.EncodeToString(h.Sum(nil)), nil
}

func HasIntersection(a []string, b []string) bool  {
	if len(a) == 0 || len(b) == 0 {
		return false
	}

	t := strings.Join(b, "%") + "%"
	for _,v := range a {
		if strings.Contains(t, v + "%") {
			return true
		}
	}

	return false
}

type ScanDirCallback func(string, string, error)
func ScanDir(dir string, fn ScanDirCallback) {
	files, err := ioutil.ReadDir(dir)
	if err != nil {
		fn(dir, "", err)
		return
	}

	for _, f := range files {
		if f.IsDir() {
			ScanDir(path.Join(dir, f.Name()), fn)
			continue;
		}

		if f.Size() == 0 {
			continue
		}

		fn(dir, f.Name(), nil)
	}
}

func GetStrMD5Sum(text string) string {
	ctx := md5.New()
	ctx.Write([]byte(text))
	return hex.EncodeToString(ctx.Sum(nil))
}

func GetFileContentType(out *os.File) (string, error) {
	// Only the first 512 bytes are used to sniff the content type.
	buffer := make([]byte, 512)

	_, err := out.Read(buffer)
	if err != nil {
		return "", err
	}

	// Use the net/http package's handy DectectContentType function. Always returns a valid
	// content-type by returning "application/octet-stream" if no others seemed to match.
	contentType := http.DetectContentType(buffer)

	return contentType, nil
}