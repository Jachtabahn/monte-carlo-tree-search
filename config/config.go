package config

var (
	Float = map[string]float32{
		"komi": 5.5}

	Int = map[string]int{
		"boardsize": 5,
		"max_game_length": 50,
		"exploration_length": 3,
		"predict_batch_size": 2,
		"num_examples_per_file": 14,
		"nsims": 300,
		"random_seed": 3,
		"history_size": 4}
)

const (
    PolicyScoreFactor = float32(1.0)
)
