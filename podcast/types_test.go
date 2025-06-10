package podcast

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCreateHostMap(t *testing.T) {
	hosts := []Host{
		{Name: "Alice", Gender: "female", Voice: "nova", Character: "Tech expert"},
		{Name: "Bob", Gender: "male", Voice: "echo", Character: "Economist"},
		{Name: "Charlie", Gender: "male", Voice: "onyx", Character: "Developer"},
	}

	hostMap := CreateHostMap(hosts)

	assert.Len(t, hostMap, 3)

	alice, ok := hostMap["Alice"]
	assert.True(t, ok)
	assert.Equal(t, "female", alice.Gender)
	assert.Equal(t, "nova", alice.Voice)

	bob, ok := hostMap["Bob"]
	assert.True(t, ok)
	assert.Equal(t, "male", bob.Gender)
	assert.Equal(t, "echo", bob.Voice)

	charlie, ok := hostMap["Charlie"]
	assert.True(t, ok)
	assert.Equal(t, "male", charlie.Gender)
	assert.Equal(t, "onyx", charlie.Voice)
}

func TestCreateHostMapEmpty(t *testing.T) {
	var hosts []Host
	hostMap := CreateHostMap(hosts)
	assert.Empty(t, hostMap)
	assert.NotNil(t, hostMap)
}

func TestCreateHostMapSingleHost(t *testing.T) {
	hosts := []Host{
		{Name: "Solo", Gender: "female", Voice: "nova", Character: "Expert"},
	}

	hostMap := CreateHostMap(hosts)

	assert.Len(t, hostMap, 1)
	info, ok := hostMap["Solo"]
	assert.True(t, ok)
	assert.Equal(t, "female", info.Gender)
	assert.Equal(t, "nova", info.Voice)
}