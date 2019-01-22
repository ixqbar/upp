package upp

import (
	"encoding/xml"
	"errors"
	"fmt"
	"os"
)

type RsyncConfig struct {
	Address string `xml:"address"`
	Repertory string `xml:"repertory"`
	Url string `xml:"url"`
	Key string `xml:"key"`
	AllowContentType []string `xml:"allow>contentType"`
}

var rsyncConfig *RsyncConfig

func ParseXmlConfig(path string) (*RsyncConfig, error) {
	if len(path) == 0 {
		return nil, errors.New("not found configure xml file")
	}

	n, err := GetFileSize(path)
	if  err !=nil || n == 0 {
		return nil, errors.New("not found configure xml file")
	}

	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	rsyncConfig = &RsyncConfig{}

	data := make([]byte, n)

	m, err := f.Read(data)
	if err != nil {
		return nil, err
	}

	if int64(m) != n {
		return nil, errors.New(fmt.Sprintf("expect read configure xml file size %d but result is %d", n, m))
	}

	err = xml.Unmarshal(data, &rsyncConfig)
	if err != nil {
		return nil, err
	}

	r, err := CheckFileIsDirectory(rsyncConfig.Repertory)
	if !r {
		return nil, err
	}

	Logger.Printf("parse config xml %s", rsyncConfig)

	return rsyncConfig, nil
}