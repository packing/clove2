package base

type ChannelQueue chan interface{}

func (c ChannelQueue) Close() {
	defer func() {
		LogPanic(recover())
	}()

	for {
		select {
		case _ = <-c:
		default:
			close(c)
			break
		}
	}
	//close(c)
}
