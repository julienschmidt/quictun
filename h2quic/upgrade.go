package h2quic

import (
	"errors"

	quic "github.com/lucas-clemente/quic-go"
)

var noKnownUpgradeProtocol = errors.New("no known upgrade protocol")

// connectionUpgrade indicates that the connection has been upgraded to the
// protocol set within.
type connectionUpgrade struct {
	protocol string
}

func (c *connectionUpgrade) Error() string {
	return "connection has been upgraded to " + c.protocol
}

type UpgradeHandler func(quic.Session)

var upgradeHandlers = map[string]UpgradeHandler{}

func RegisterUpgradeHandler(protocol string, handler UpgradeHandler) {
	upgradeHandlers[protocol] = handler
}
