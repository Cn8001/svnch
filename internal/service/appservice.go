package service

import "github.com/Cn8001/svnch/internal/appinfo"

type AppService struct {
	info appinfo.Info
}

func NewAppService(info appinfo.Info) *AppService { return &AppService{info: info.WithBuild()} }

func (s *AppService) About() appinfo.Info { return s.info }
