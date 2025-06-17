package filter

import (
	"hash/fnv"
	url2 "net/url"
	"strings"
	"sync"
)

type UrlSet struct {
	Length int
	Set    map[uint64]bool
	mu     sync.Mutex
}

func (s *UrlSet) Add(url string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Set[HashUrl(url)] = true
	s.Length++
}

func (s *UrlSet) Contains(url string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, ok := s.Set[HashUrl(url)]

	return ok
}

func (s *UrlSet) Size() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.Length
}

func HashUrl(url string) uint64 {
	h := fnv.New64a()
	h.Write([]byte(url))

	return h.Sum64()
}

func NormalizeUrl(url string) (string, error) {
	normalizedUrl, err := url2.Parse(url)
	if err != nil {
		return "", err
	}

	normalizedUrl.Fragment = ""

	// LITTLE LOGIC TO PREVENT OVER NORMALIZATION OF URLs
	keep := map[string]bool{
		"page": true,
		"lang": true,
		"id":   true,
	}
	q := normalizedUrl.Query()
	for k, _ := range q {
		if !keep[k] {
			q.Del(k)
		}
	}
	normalizedUrl.RawQuery = q.Encode()

	normalizedUrl.Scheme = strings.ToLower(normalizedUrl.Scheme)
	normalizedUrl.Host = strings.ToLower(normalizedUrl.Host)

	if normalizedUrl.Path == "" {
		normalizedUrl.Path = "/"
	}

	return normalizedUrl.String(), nil
}
