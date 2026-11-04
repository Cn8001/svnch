package service

import (
	"github.com/Cn8001/svnch/internal/appinfo"
	"github.com/Cn8001/svnch/internal/explorer"
)

type AppService struct {
	info   appinfo.Info
	launch explorer.Launch
}

// NewAppService takes launch, what svnch was started to do, as an Explorer menu entry asks with --action and a path.
func NewAppService(info appinfo.Info, launch explorer.Launch) *AppService {
	return &AppService{info: info.WithBuild(), launch: launch}
}

func (s *AppService) About() appinfo.Info { return s.info }

func (s *AppService) Launch() explorer.Launch { return s.launch }
