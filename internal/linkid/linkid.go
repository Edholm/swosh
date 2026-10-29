// Package linkid makes the ids of new links. They are three words, like
// "brave-otter-sings", so that people can read them out and type them
// without mixing up characters.
package linkid

import (
	"crypto/rand"
	"math/big"
)

// New returns a random adjective, noun and verb joined by hyphens. The word
// lists give about 7.5 million ids; links that are taken are retried by the
// caller.
func New() string {
	return pick(adjectives) + "-" + pick(nouns) + "-" + pick(verbs)
}

func pick(words []string) string {
	i, _ := rand.Int(rand.Reader, big.NewInt(int64(len(words)))) // never fails
	return words[i.Int64()]
}
