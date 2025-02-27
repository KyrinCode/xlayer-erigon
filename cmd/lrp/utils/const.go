package utils

const (
	DEFAULT_DESTINATION_DIR          = ".lrp"
	DEFAULT_SOURCE_MAINNET_DATA_PATH = "mainnet/seq"
	DEFAULT_EXTERNAL_DATASTREAM_PATH = "mainnet/seq/data-stream"

	REPO_NAME       = "xlayer-erigon"
	LRP_CONFIG_FILE = "lrp.config.yaml"
	UNWIND_LOG      = "unwind.log"
	REPLAY_LOG      = "replay.log"
	UNWOUND_REPO    = "unwound-repo"

	// commands
	LRP_CONFIG         = "lrp-config"
	LRP_MAINNET_UNWIND = "lrp-mainnet-unwind"
	LRP_MAINNET_REPLAY = "lrp-mainnet-replay"
	LRP_STOP           = "lrp-stop"

	REPLAY_STOP_SIGN = "Resequencing completed"
)
