package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParseDeployBsaeHref(t *testing.T) {
	u := "/default/controller/v1/bid-1234/deploymentBase/acid-5678"
	bid, acid := parseDeployBsaeHref(u)
	assert.Equal(t, "bid-1234", bid)
	assert.Equal(t, "acid-5678", acid)
}
