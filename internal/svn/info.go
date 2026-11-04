package svn

import (
	"encoding/xml"
	"fmt"
	"time"
)

type Commit struct {
	Revision int64     `xml:"revision,attr" json:"revision"`
	Author   string    `xml:"author" json:"author"`
	Date     time.Time `xml:"date" json:"date"`
}

type Info struct {
	Path            string `xml:"path,attr" json:"path"`
	Kind            string `xml:"kind,attr" json:"kind"` // file or dir
	Revision        int64  `xml:"revision,attr" json:"revision"`
	URL             string `xml:"url" json:"url"`
	RelativeURL     string `xml:"relative-url" json:"relativeUrl"`
	RepositoryRoot  string `xml:"repository>root" json:"repositoryRoot"`
	RepositoryUUID  string `xml:"repository>uuid" json:"repositoryUuid"`
	WorkingCopyRoot string `xml:"wc-info>wcroot-abspath" json:"workingCopyRoot"` // empty for a URL target
	Schedule        string `xml:"wc-info>schedule" json:"schedule"`
	Depth           string `xml:"wc-info>depth" json:"depth"`
	LastCommit      Commit `xml:"commit" json:"lastCommit"`
}

func InfoArgs(target string) []string {
	return []string{"--xml", "--", pegSafe(target)}
}

func ParseInfo(data []byte) (Info, error) {
	var doc struct {
		Entries []Info `xml:"entry"`
	}
	if err := xml.Unmarshal(data, &doc); err != nil {
		return Info{}, fmt.Errorf("parse svn info: %w", err)
	}
	if len(doc.Entries) == 0 {
		return Info{}, fmt.Errorf("parse svn info: no entry in output")
	}
	return doc.Entries[0], nil
}
