// Package flex is the IBKR Flex Web Service client, its report parser and the loader of
// $OT_HOME/flex.toml. It only asks Flex for reports: it cannot place, change or cancel an order.
// It reads no environment; the token comes in through sources.FlexConfig. Host and paths are
// constants, no redirect is followed, no address from an answer is used, and neither the token, the
// query id nor an address is put in an error.
package flex
