package codec

import (
	"bytes"
	"os"
	"sync"

	"github.com/packing/clove2/base"
)

type ConfigurationReader struct {
	path            string
	reader          *CloveMapReader
	onceForLazyLoad sync.Once
}

func LoadConfiguration(path string) *ConfigurationReader {
	c := new(ConfigurationReader)
	c.path = path
	c.reader = nil
	return c
}

func (c *ConfigurationReader) load() {
	defer func() {
		base.LogPanic(recover())
	}()
	c.onceForLazyLoad.Do(func() {
		confPtr, err := os.Open("conf.json")
		if err != nil {
			return
		}
		buf := make([]byte, 512)
		ctx := new(bytes.Buffer)
		for {
			r, e := confPtr.Read(buf)
			if e != nil && r <= 0 {
				break
			}
			ctx.Write(buf[:r])
		}
		_ = confPtr.Close()

		err, configData, _ := GetCodecManager().FindCodec("json").Decoder.Decode(ctx.Bytes())
		c.reader = CreateMapReader(configData.(CloveMap))
	})
}

func (c *ConfigurationReader) ReadString(key string) string {
	c.load()
	return c.reader.StrValueOf(key, "")
}

func (c *ConfigurationReader) ReadInt(key string) int64 {
	c.load()
	return c.reader.IntegerNumberOf(key, 0)
}

func (c *ConfigurationReader) ReadBool(key string) bool {
	c.load()
	return c.reader.BoolValueOf(key)
}

func (c *ConfigurationReader) ReadFloat(key string) float64 {
	c.load()
	return c.reader.FloatValueOf(key, 0.0)
}
