# Monte Carlo tree search

[Monte Carlo tree search](https://en.wikipedia.org/wiki/Monte_Carlo_tree_search) is a known concept. But [DeepMind](https://deepmind.com/research/publications/mastering-game-go-without-human-knowledge) used this concept groundbreakingly to develop a super human AI in the game of Go without using human game protocols.

There is the main [Monte Carlo Tree Search code](https://github.com/Jachtabahn/monte-carlo-tree-search/blob/master/treesearch/searcher.go). In that source file, the function `simulate` on the bottom is the most important one: This is one stitch of the Monte Carlo Tree. This is one extension of the cleverly chosen Monte Carlo Tree branch. This is one thought of the agent.

Another interesting part of the code is the [predictor service](https://github.com/Jachtabahn/monte-carlo-tree-search/blob/master/predictor/predictor.go). This is a separate [goroutine](https://gobyexample.com/goroutines) that takes prediction tasks off a channel and answers them with neural network outputs on another channel. The prediction tasks are basically the game board states and the network outputs are game action probabilities. According to those probabilities, an action is drawn and taken.

To compute those neural network outputs, a [Go binding to TensorFlow](https://pkg.go.dev/github.com/tensorflow/tensorflow/tensorflow/go?tab=doc) is used.
