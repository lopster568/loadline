package tokens

import (
	"sync"

	tiktoken "github.com/pkoukk/tiktoken-go"
	tiktokenloader "github.com/pkoukk/tiktoken-go-loader"
)

// OpenAIEncoding is the pinned encoding from methodology 1.6.
const OpenAIEncoding = "o200k_base"

// OpenAI counts with a local tiktoken encoding. It never reaches the network:
// the BPE ranks come from the embedded offline loader.
type OpenAI struct {
	once sync.Once
	enc  *tiktoken.Tiktoken
	err  error
}

// NewOpenAI builds the local counter.
func NewOpenAI() *OpenAI { return &OpenAI{} }

func (o *OpenAI) load() {
	o.once.Do(func() {
		tiktoken.SetBpeLoader(tiktokenloader.NewOfflineLoader())
		o.enc, o.err = tiktoken.GetEncoding(OpenAIEncoding)
	})
}

// Count returns the token count of text under o200k_base.
func (o *OpenAI) Count(text string) (int, error) {
	o.load()
	if o.err != nil {
		return 0, o.err
	}
	return len(o.enc.Encode(text, nil, nil)), nil
}

// Stamp returns the cell provenance for an OpenAI count.
func (o *OpenAI) Stamp() Stamp {
	return Stamp{Encoding: OpenAIEncoding, MeasuredAt: nowUTC()}
}
