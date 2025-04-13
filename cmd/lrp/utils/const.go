package utils

import "time"

const (
	DEFAULT_DESTINATION_DIR = "/data/.lrp"

	DEFAULT_SAMPLE_INTERVAL = 10 * time.Second
	MIN_SMAPLE_INTERVAL     = time.Second
	DEFAULT_CHAINDATA_LIMIT = 100 * 1024 * 1024 * 1024
	LRP_MAINNET_CONFIG_FILE = "xlayer-erigon/test/lrp/xlayerconfig-mainnet.yaml"

	// default test params
	DEFAULT_PROCESS_COUNT            = 1
	DEFAULT_USER                     = "default"
	DEFAULT_USE_EXTERNAL_DATASTREAM  = true
	DEFAULT_SOURCE_MAINNET_DATA_PATH = "mainnet"
	DEFAULT_EXTERNAL_DATASTREAM_PATH = "mainnet/seq/data-stream"

	REPO_NAME       = "xlayer-erigon"
	LRP_CONFIG_FILE = "lrp.config.yaml"
	UNWIND_LOG      = "unwind.log"
	REPLAY_LOG      = "replay.log"
	UNWOUND_REPO    = "unwound-repo"
	HISTORY_FOLDER  = "history"
	OPTIONS_JSON    = "options.json"

	// commands
	LRP_CONFIG                 = "lrp-config"
	LRP_MAINNET_UNWIND         = "lrp-mainnet-unwind"
	LRP_MAINNET_REPLAY         = "lrp-mainnet-replay"
	LRP_MAINNET_DATA_COMPACT   = "lrp-mainnet-data-compact"
	LRP_MAINNET_REPLAY_VMTOUCH = "lrp-mainnet-replay-vmtouch"
	LRP_STOP                   = "lrp-stop"
	LRP_CLEAN                  = "lrp-clean"
	LRP_MAINNET_REPLAY_PAUSE   = "lrp-mainnet-replay-pause"

	// stop sign
	REPLAY_STOP_SIGN = "Resequencing completed"

	// comment out code
	COMMENT_OUT_FILE    = "cmd/integration/root.go"
	COMMENT_TARGET_LINE = `opts = opts.Accede()`
)
