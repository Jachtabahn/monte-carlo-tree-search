package config

var (
	Float = map[string]float32 {
		"komi": 5.5}

	Int = map[string]int {
		"boardsize": 5,
		"max_game_length": 50,
		"exploration_length": 3,
		"predict_batch_size": 2,
		"num_examples_per_file": 200,
		"nsims_per_goroutine": 60,
		"random_seed": 3,
		"history_size": 4}

	String = map[string]string {
		"exp_prefix": "exp",
		"model_path": "/home/tischler/Software/tischler/main/out/models/uibam-10"}
)

const (
    PolicyScoreFactor = float32(1.0)
)
