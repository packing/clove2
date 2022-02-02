package base

type ChannelQueue chan interface{}

func (c ChannelQueue) Close() {
	defer func() {
		LogVerbose("ChannelQueue closed.")
		LogPanic(recover())
	}()

	for {
		select {
		case _ = <-c:
		default:
			close(c)
			return
		}
	}
	//close(c)
}
