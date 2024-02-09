package modelchecker

import (
	"fizz/proto"
	"github.com/jayaprabhakar/fizzbee/lib"
)

func ReadOptionsFromYaml(filename string) (*proto.StateSpaceOptions, error) {
	msg := &proto.StateSpaceOptions{}
	err := lib.ReadProtoFromFile(filename, msg)
	if err != nil {
		return nil, err
	}
	if msg.Options.MaxConcurrentActions == 0 {
		msg.Options.MaxConcurrentActions = msg.Options.MaxActions
	}
	return msg, err
}