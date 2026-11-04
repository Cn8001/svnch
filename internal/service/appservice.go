package service

import "github.com/Cn8001/svnch/internal/appinfo"

type AppService struct {
	info  appinfo.Info
	start string
}

// NewAppService takes start, the folder svnch was launched with, as Explorer's "Open with svnch" does.
func NewAppService(info appinfo.Info, start string) *AppService {
	return &AppService{info: info.WithBuild(), start: start}
}

func (s *AppService) About() appinfo.Info { return s.info }

func (s *AppService) StartFolder() string { return s.start }
