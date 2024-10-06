package configs

import "github.com/mgp-systems/internal-fabrik8-api/configs"

const DefaultK1Version = configs.DefaultK1Version

//nolint:gochecknoglobals
var (
	K1Version             = configs.K1Version
	ReadConfig            = configs.ReadConfig
	InitializeViperConfig = configs.InitializeViperConfig
)
