package optioneod

import (
	_ "embed"
	"errors"

	"github.com/andrewrong/optdesk/internal/broker"
	"github.com/andrewrong/optdesk/internal/clock"
	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/jobs"
	"github.com/andrewrong/optdesk/internal/store"
)

var _ jobs.NewOptionEodFunc = NewJP

//go:embed job_jp.toml
var manifestJPBytes []byte

// NewJP builds the job option_eod_jp from the embedded job_jp.toml.
func NewJP(b broker.Broker, res store.InstrumentResolver, w store.OptionEodWriter, clk clock.Clock) (contracts.Job, error) {
	if b == nil || res == nil || w == nil || clk == nil {
		return nil, errors.New("optioneod: broker, resolver, writer and clock are required")
	}
	return build(manifestJPBytes, b, res, w, clk)
}
