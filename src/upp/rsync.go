package upp

import (
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path"
	"strings"
	"sync"
	"syscall"
	"time"
)

type TTask struct {
	file         string
	name         string
	size         int64
	md5sum       string
	relativePath string
	id string
	report_id string
}

func NewTask(p, name string) (*TTask, error) {
	taskPath := path.Join(p, name)
	n, err := GetFileSize(taskPath)
	if err != nil {
		Logger.Print(err)
		return nil, err
	}

	md5sum, err := GetFileMD5sum(taskPath)
	if err != nil {
		Logger.Print(err)
		return nil, err
	}

	rootDir := path.Dir(path.Join(rsyncConfig.Repertory, "todo.md"))

	taskRelativePath := path.Clean(taskPath[len(rootDir) + 1:len(taskPath) - len(name)])
	if taskRelativePath == "." {
		taskRelativePath = ""
	}

	return &TTask{
		file:taskPath,
		name:name,
		size:n,
		md5sum:md5sum,
		relativePath:taskRelativePath,
		id:name[:(len(name) - len(path.Ext(name)))],
		report_id:path.Join(taskRelativePath, md5sum + path.Ext(name)),
	}, nil
}

type TSync struct {
	sync.Mutex
	outStopChannel  chan int
	catchStopSignal bool
	conn            net.Conn
	buffer          []byte
	tryConnect      bool
}

func (obj *TSync) Start() {
	successNum := 0
	ScanDir(rsyncConfig.Repertory, func(p, f string, e error) {
		if e != nil {
			Logger.Print(e)
			return
		}

		if len(f) == 0 {
			return
		}

		tf, err := os.Open(path.Join(p, f))
		if err != nil {
			Logger.Print(err)
			return
		}
		defer tf.Close()

		// Get the content
		contentType, err := GetFileContentType(tf)
		if err != nil {
			Logger.Print(err)
			return
		}

		if InStringArray(contentType, rsyncConfig.AllowContentType) == false {
			Logger.Printf("found exclude file %s contentType %s", path.Join(p, f), contentType)
			return
		}

		t, err := NewTask(p, f)
		if err != nil {
			Logger.Print(err)
			return
		}

		//Logger.Printf("found file %v", t)
		if ok, _ := obj.Sync(t, 3); ok {
			successNum++
		}
	})

	Logger.Printf("success report num %d", successNum)

	obj.outStopChannel <- 1
}

func (obj *TSync) Stop() {
	obj.catchStopSignal = true
}

func (obj *TSync) Sync(t *TTask, maxNum int) (bool, error) {
	if len(rsyncConfig.Url) > 0 {
		requestURL := fmt.Sprintf("%s?id=%s&action=check", rsyncConfig.Url, t.id)
		response, err := http.Get(requestURL)
		if err != nil {
			Logger.Printf("check website report %s fail %s", t.id, err)
			return false, err
		} else {
			defer response.Body.Close()
			contents, err := ioutil.ReadAll(response.Body)
			if err != nil || string(contents) != "ok" {
				Logger.Printf("check report=%s fail with message `%s`", t.id, string(contents))
				return false, err
			}
		}
	}

	loop := 0

	for {
		if obj.catchStopSignal {
			Logger.Printf("sync %s to server aborted", t.name)
			break
		}

		if loop > maxNum {
			return false, fmt.Errorf("sync %s overflow max num %d", t.name, maxNum)
		}

		loop++

		ok, err := obj.SyncExecute(t)
		if err != nil {
			if err != io.EOF {
				Logger.Print(err)
			}
			continue
		}

		if !ok {
			Logger.Printf("sync %s again", t.name)
			continue
		}

		if len(rsyncConfig.Url) > 0 {
			requestURL := fmt.Sprintf("%s?id=%s&report_id=%s&hash=%s", rsyncConfig.Url, t.id, t.report_id, GetStrMD5Sum(fmt.Sprintf("%s-%s-%s", t.id, t.report_id, rsyncConfig.Key)))
			response, err := http.Get(requestURL)
			if err != nil {
				Logger.Printf("update website reprot=%s fail with message `%s`", t.id, err)
			} else {
				defer response.Body.Close()
				contents, err := ioutil.ReadAll(response.Body)
				if err != nil || string(contents) != "ok" {
					Logger.Printf("update website report=%s fail with message `%s`", t.id, string(contents))
				} else {
					return true, nil
				}
			}
		}

		return false, nil
	}

	return false, nil
}

func (obj *TSync) Connect() error {
	conn, err := net.Dial("tcp", rsyncConfig.Address)
	if err != nil {
		Logger.Printf("connecting target server failed %s", err)
		return err
	}

	obj.conn = conn
	obj.tryConnect = false

	Logger.Printf("connecting target server %s success", rsyncConfig.Address)

	return nil
}

func (obj *TSync) SyncExecute(t *TTask) (bool, error) {
	obj.Lock()
	defer obj.Unlock()

	if obj.tryConnect {
		err := obj.Connect()
		if err != nil {
			Logger.Printf("reconnect target server failed %s", err)
			return false, err
		}
		obj.tryConnect = false
	}

	f, err := os.Open(t.file)
	if err != nil {
		Logger.Printf("open file %s failed %v", t.name, err)
		return false, err
	}

	defer f.Close()

	targetFileSuffix := fmt.Sprintf("%s@%d@%s@%s\r\n", t.report_id, t.size, t.md5sum, t.relativePath)
	obj.WriteAll([]byte(targetFileSuffix))

	//CONTINUE\r\n
	//ALL_SAME\r\n

	message, err := obj.ReadAll(10)
	if err == nil && len(message) >= 10 {
		rr := strings.Trim(string(message), "\r\n")
		if rr == "ALL_SAME" {
			Logger.Printf("sync %s -> %s success", t.name, t.report_id)
			return true, nil
		}

		if rr != "CONTINUE" {
			obj.tryConnect = true
			Logger.Printf("read sync %s to server header response [%s] failed %s", t.name, rr, err)
			return false, errors.New("error sync header response")
		}
	} else {
		obj.tryConnect = true
		Logger.Printf("read sync %s to server header response failed %s", t.name, err)
		return false, err
	}

	buf := make([]byte, 1024)
	total := 0
	for {
		nr, er := f.Read(buf)
		if er != nil {
			break
		}

		if nr > 0 {
			nw, ew := obj.conn.Write(buf[0:nr])
			if ew != nil {
				break
			}

			total += nw
		}

		if int64(total) >= t.size {
			break
		}
	}

	//OK\r\n
	message, err = obj.ReadAll(4)
	if err == nil && len(message) >= 4 {
		rr := strings.Trim(string(message), "\r\n")
		if rr == "OK" {
			Logger.Printf("sync %s -> %s success", t.name, t.report_id)
			return true, nil
		}
	}

	obj.tryConnect = true
	Logger.Printf("read sync %s to server failed %s", t.name, err)

	return false, errors.New("error sync header response")
}

func (obj *TSync) ReadAll(minLen int) ([]byte, error) {
	readLen := 0

	for {
		obj.conn.SetWriteDeadline(time.Now().Add(time.Second * time.Duration(30)))
		n, err := obj.conn.Read(obj.buffer[readLen:])
		if err != nil {
			Logger.Printf("read server response failed %s", err)
			break
		}

		readLen += n
		if readLen >= minLen {
			break
		}
	}

	return obj.buffer[:readLen], nil
}

func (obj *TSync) WriteAll(message []byte) bool {
	totalLen := len(message)
	writeLen := 0

	for {
		obj.conn.SetWriteDeadline(time.Now().Add(time.Second * time.Duration(30)))
		n, err := obj.conn.Write(message[writeLen:])
		if err != nil {
			Logger.Printf("sync write data failed %s", err)
			return false
		}

		writeLen += n
		if writeLen >= totalLen {
			return true
		}
	}
}

func NewTSync(stopChannel chan int) *TSync {
	return &TSync{
		outStopChannel:  stopChannel,
		catchStopSignal: false,
		tryConnect:true,
		buffer: make([]byte, 1024),
	}
}

func Run() {
	stopChannel := make(chan int)
	sigs := make(chan os.Signal)
	signal.Notify(sigs, syscall.SIGTERM, syscall.SIGINT)

	tsync := NewTSync(stopChannel)

	go func() {
		<-sigs
		tsync.Stop()
	}()

	go tsync.Start()

	<-stopChannel

	Logger.Print("done")
}
