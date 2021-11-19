package base

type ChannelQueue chan interface{}

func (c ChannelQueue) Close() {
	defer func() {
		LogPanic(recover())
	}()
	close(c)
}
