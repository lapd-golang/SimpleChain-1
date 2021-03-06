package network

type GoAwayReason uint8
const (
	No_Reason GoAwayReason = iota	// no reason to go away
	Self 							// the connection is to itself
	Duplicate 						// the connection is redundant
	Wrong_Chain 					// the peer's chain id doesn't match
	Wrong_Version 					// the peer's network version doesn't match
	Forked 							// the peer's irreversible blocks are different
	Unlinkable 						// the peer sent a block we couldn't use
	Bad_Transaction 				// the peer sent a transaction that failed verification
	Validation 						// the peer sent a block that failed validation
	Benign_Other 					// reasons such as a timeout. not fatal but warrant resetting
	Fatal_Other 					// a catch-all for errors we don't have discriminated
	Authentication 					// peer failed authenicatio
)

type IdListModes byte
const (
	None IdListModes = iota
	Catch_Up
	Last_Irr_Catch_Up
	Normal
)

type MessageTypes byte
const (
	Handshake MessageTypes = iota
	ChainSize
	GoAway
	Time
	Notice
	Request
	SyncRequest
	SignedBlock
	PackedTransaction
)

const TCP  = "tcp"
const MessageHeaderSize = 4
const PeerAuthenticationInterval = 1 //second
const SyncReqSpan = 100 //second
const MaxTransactionTime = 30 // ms

type MessageType interface {}
type MessageHeader struct {
	Type 	MessageTypes // 1 byte
	Length  uint32  	// 4 bytes
}
type Message struct {
	Header MessageHeader
	Content MessageType
}