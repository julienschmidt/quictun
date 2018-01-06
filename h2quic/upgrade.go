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

// UpgradeHandler is a function which can perform an upgrade to another protocol
// by modifying a given QUIC session.
type UpgradeHandler func(quic.Session)

// map of registered UpgradeHandlers
var upgradeHandlers = map[string]UpgradeHandler{}

// RegisterUpgradeHandler registers a handler function for the given protocol
// identifier, such as "PROT/1.2".
func RegisterUpgradeHandler(protocol string, handler UpgradeHandler) {
	upgradeHandlers[protocol] = handler
}
