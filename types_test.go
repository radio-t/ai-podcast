package main

import (
	"testing"

	"github.com/radio-t/ai-podcast/podcast"
	"github.com/stretchr/testify/assert"
)

func TestCreateHostMap(t *testing.T) {
	hosts := []podcast.Host{
		{Name: "TestHost1", Gender: "male", Voice: "echo", Character: "Skeptical tech expert"},
		{Name: "TestHost2", Gender: "female", Voice: "nova", Character: "Analytical economist"},
		{Name: "TestHost3", Gender: "male", Voice: "onyx", Character: "Enthusiastic developer"},
	}

	hostMap := podcast.CreateHostMap(hosts)

	// check that all hosts are in the map
	assert.Len(t, hostMap, 3)

	// check first host
	info1, ok := hostMap["TestHost1"]
	assert.True(t, ok)
	assert.Equal(t, "male", info1.Gender)
	assert.Equal(t, "echo", info1.Voice)

	// check second host
	info2, ok := hostMap["TestHost2"]
	assert.True(t, ok)
	assert.Equal(t, "female", info2.Gender)
	assert.Equal(t, "nova", info2.Voice)

	// check third host
	info3, ok := hostMap["TestHost3"]
	assert.True(t, ok)
	assert.Equal(t, "male", info3.Gender)
	assert.Equal(t, "onyx", info3.Voice)
}

func TestCreateHostMapEmpty(t *testing.T) {
	var hosts []podcast.Host
	hostMap := podcast.CreateHostMap(hosts)
	assert.Len(t, hostMap, 0)
}
