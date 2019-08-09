package config

var (
	Float = map[string]float32{
		"komi": 5.5}

	Int = map[string]int{
		"boardsize": 5,
		"max_game_length": 20,
		"exploration_length": 3,
		"predict_batch_size": 2,
		"num_examples_per_file": 41,
		"nsims_per_goroutine": 600,
		"random_seed": 3,
		"num_eval_games": 1,
		"history_size": 4}

	String = map[string]string{
		"exp_prefix": "exp",
		"record_prefix": "sgf",
		"commands_path": "commands",
		"model_path": "/home/tischler/Software/tischler/main/out/mod/uibam-tf"}
)

const (
    PolicyScoreFactor = float32(1.0)
    BLACK = 1
    WHITE = 2
    ModelTag = "gogame"
)
