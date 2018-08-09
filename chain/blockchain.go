package chain

import (
	"blockchain/database"
	bytes2 "bytes"
	"log"
	"fmt"
	"blockchain/crypto"
	"crypto/sha256"
)

type BlockChainConfig struct {
	BlocksDir string
	StateDir string
	StateSize uint64
	ReversibleCacheSize uint64
	Genesis GenesisState
}

type PendingState struct {
	PendingBlockState *BlockState
	Actions []ActionReceipt
	BlockStatus BlockStatus
}

type BlockChain struct {
	Config BlockChainConfig
	DB database.Database
	ReversibleBlocks []*database.ReversibleBlockObject
	Blog database.BlockLog
	Pending *PendingState
	Head *BlockState
	ForkDatabase database.ForkDatabase
	Authorization AuthorizationManager
	ChainId SHA256Type
	Replaying bool
	UnAppliedTransactions map[SHA256Type]*TransactionMetaData
}

func NewBlockChain() BlockChain {
	bc := BlockChain{}
	if bc.Head == nil {

	}
	return bc
}

func max(a, b uint32) uint32 {
	if a > b {
		return a
	}
	return b
}

func (bc *BlockChain) initializeForkDatabase() {
	fmt.Println("Initializing new blockchain with genesis state")
	pub, _ := crypto.NewPublicKey(bc.Config.Genesis.InitialKey)
	producerKey := ProducerKey{
		DEFAULT_PRODUCER_NAME,
		pub,
	}
	initialSchedule := ProducerScheduleType{
		Version: 0,
		Producers: []ProducerKey{producerKey},
	}
	genHeader := BlockHeaderState{}
	genHeader.ActiveSchedule = initialSchedule
	genHeader.PendingSchedule = initialSchedule
	initialScheduleBytes, _ := MarshalBinary(initialSchedule)
	genHeader.PendingScheduleHash = sha256.Sum256(initialScheduleBytes)
	genHeader.Header.Timestamp = bc.Config.Genesis.InitialTimestamp
	genHeader.Id = genHeader.Header.Id()
	genHeader.BlockNum = uint64(genHeader.Header.BlockNum())
	bc.Head.BlockHeaderState = genHeader
	bc.Head.Block.SignedBlockHeader = genHeader.Header
	bc.ForkDatabase = database.ForkDatabase{}
	bc.ForkDatabase.BlockStates = []*BlockState{bc.Head}
}

func (bc *BlockChain) initializeDatabase() {
	bc.DB = database.Database{}
	taposBlockSumary := database.BlockSummaryObject{}
	taposBlockSumary.BlockId = bc.Head.Id
	bc.DB.BlockSummaryObjects = []*database.BlockSummaryObject{&taposBlockSumary}
}

func (bc *BlockChain) LastIrreversibleBlockNum() uint32 {
	return max(bc.Head.DPOSIrreversibleBlockNum, bc.Head.BFTIrreversibleBlockNum)
}

func (bc *BlockChain) LastIrreversibleBlockId() *SHA256Type {
	libNum := bc.LastIrreversibleBlockNum()
	taposBlockSummary := bc.DB.GetBlockSummaryObject(libNum)
	if taposBlockSummary != nil {
		return &taposBlockSummary.BlockId
	}
	block := bc.FetchBlockByNum(libNum)
	if block != nil {
		blockId := block.Id()
		return &blockId
	}
	return nil
}

func (bc *BlockChain) GetBlockIdForNum(blockNum uint32) SHA256Type {
	blockState := bc.ForkDatabase.GetBlockInCurrentChainIdNum(blockNum)
	if blockState != nil {
		return blockState.Id
	}
	signedBlock := bc.Blog.ReadBlockByNum(blockNum)
	return signedBlock.Id()
}

func (bc *BlockChain) FetchBlockById(id SHA256Type) *SignedBlock {
	state := bc.ForkDatabase.GetBlock(id)
	if state != nil {
		return state.Block
	}
	block := bc.FetchBlockByNum(NumFromId(id))
	if block != nil && bytes2.Equal(block.Id()[:], id[:]) {
		return block
	}
	return nil
}

func (bc *BlockChain) FetchBlockByNum(num uint32) *SignedBlock {
	blockState := bc.ForkDatabase.GetBlockInCurrentChainIdNum(num)
	if blockState != nil {
		return blockState.Block
	}
	signedBlock := bc.Blog.ReadBlockByNum(num)
	return signedBlock
}

func (bc *BlockChain) PendingBlocKState() *BlockState {
	if bc.Pending != nil {
		return bc.Pending.PendingBlockState
	}
	return nil
}

func (bc *BlockChain) AbortBlock() {
	if bc.Pending != nil {
		for _, trx := range bc.Pending.PendingBlockState.Trxs {
			bc.UnAppliedTransactions[trx.Signed] = trx
		}
		bc.Pending = nil
	}
}

func (bc *BlockChain) StartBlock(when uint64, confirmBlockCount uint16, s BlockStatus) {
	if bc.Pending == nil {
		log.Fatal("pending block should be not nil")
	}
	// generate pending block
	bc.Pending.BlockStatus = s
	bc.Pending.PendingBlockState = NewBlockState(bc.Head.BlockHeaderState, when)
	bc.Pending.PendingBlockState.InCurrentChain = true
	bc.Pending.PendingBlockState.SetConfirmed(confirmBlockCount)
	wasPendingPromoted := bc.Pending.PendingBlockState.MaybePromotePending()
	gpo := bc.DB.GPO
	if gpo.ProposedScheduleBlockNum != nil &&
		*gpo.ProposedScheduleBlockNum <= bc.Pending.PendingBlockState.DPOSIrreversibleBlockNum &&
		len(bc.Pending.PendingBlockState.PendingSchedule.Producers) == 0 &&
		!wasPendingPromoted {
			bc.Pending.PendingBlockState.SetNewProducer(gpo.ProposedSchedule.ProducerSchedulerType())
			bc.DB.GPO.ProposedSchedule = nil
			bc.DB.GPO.ProposedScheduleBlockNum = nil
	}
	// remove expired transaction in database
	bc.ClearExpiredInputTransaction()
}

func removeTransactionObject(transactionObjects []*database.TransactionObject, index int) []*database.TransactionObject {
	return append(transactionObjects[:index], transactionObjects[index+1:]...)
}

func (bc *BlockChain) ClearExpiredInputTransaction() {
	expriedIndexes := make([]int, 0)
	now := bc.Pending.PendingBlockState.Header.Timestamp.ToTime()
	for i, obj := range bc.DB.TransactionObjects {
		if now.After(obj.Expiration) {
			expriedIndexes = append(expriedIndexes, i)
		}
	}
	for _, index := range expriedIndexes{
		bc.DB.TransactionObjects = removeTransactionObject(bc.DB.TransactionObjects, index)
	}
}

func (bc *BlockChain) FinalizeBlock() {
	if bc.Pending != nil {
		log.Fatal("it is not valid to finalize when there is no pending block")
	}
	// the part relating to transaction will be added later
	p := bc.Pending.PendingBlockState
	p.Id = p.Header.Id()
	bc.CreateBlockSumary(p.Id)
}

func (bc *BlockChain) CreateBlockSumary(id SHA256Type) {
	blockNum := NumFromId(id)
	bso := bc.DB.FindBlockSummaryObject(blockNum)
	bso.BlockId = id
}

func (bc *BlockChain) CommitBlock() {
	bc.Pending.PendingBlockState.Validated = true
	newBsp := bc.ForkDatabase.Add(bc.Pending.PendingBlockState)
	head := bc.ForkDatabase.Head
	if newBsp != head {
		log.Fatal("committed block did not become the new head in fork database")
	}
	if !bc.Replaying {
		ubo := database.ReversibleBlockObject{}
		ubo.BlockNum = bc.Pending.PendingBlockState.BlockNum
		ubo.SetBlock(bc.Pending.PendingBlockState.Block)
	}
	//accept block. Use invoked method later
	// TODO
}

func (bc *BlockChain) SignBlock(signer SignerCallBack) {
	p := bc.Pending.PendingBlockState
	p.Sign(signer)
	p.Block.SignedBlockHeader = p.Header
}