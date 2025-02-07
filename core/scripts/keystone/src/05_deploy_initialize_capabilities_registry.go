package src

import (
	"context"
	"encoding/hex"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/ethereum/go-ethereum/common"
	"google.golang.org/protobuf/proto"

	ragetypes "github.com/smartcontractkit/libocr/ragep2p/types"

	capabilitiespb "github.com/smartcontractkit/chainlink-common/pkg/capabilities/pb"
	"github.com/smartcontractkit/chainlink-common/pkg/values"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"

	helpers "github.com/smartcontractkit/chainlink/core/scripts/common"
	kcr "github.com/smartcontractkit/chainlink/v2/core/gethwrappers/keystone/generated/capabilities_registry"
)

type peer struct {
	PeerID              string
	Signer              string
	EncryptionPublicKey string
}

var (
	workflowDonPeers = []peer{
		{
			PeerID:              "12D3KooWQXfwA26jysiKKPXKuHcJtWTbGSwzoJxj4rYtEJyQTnFj",
			Signer:              "0xC44686106b85687F741e1d6182a5e2eD2211a115",
			EncryptionPublicKey: "0x0f8b6629bc26321b39dfb7e2bc096584fe43dccfda54b67c24f53fd827efbc72",
		},
		{
			PeerID:              "12D3KooWGCRg5wNKoRFUAypA68ZkwXz8dT5gyF3VdQpEH3FtLqHZ",
			Signer:              "0x0ee7C8Aa7F8cb5E08415C57B79d7d387F2665E8b",
			EncryptionPublicKey: "0x4cb8a297d524469e63e8d8a15c7682891126987acaa39bc4f1db78c066f7af63",
		},
		{
			PeerID:              "12D3KooWHggbPfMcSSAwpBZHvwpo2UHzkf1ij3qjTnRiWQ7S5p4g",
			Signer:              "0xEd850731Be048afE986DaA90Bb482BC3b0f78aec",
			EncryptionPublicKey: "0x7a9be509ace5f004fa397b7013893fed13a135dd273f7293dc3c7b6e57f1764e",
		},
		{
			PeerID:              "12D3KooWC44picK3WvVkP1oufHYu1mB18xUg6mF2sNKM8Hc3EmdW",
			Signer:              "0xD3400B75f2016878dC2013ff2bC2Cf03bD5F19f5",
			EncryptionPublicKey: "0x66dcec518809c1dd4ec5c094f651061b974d3cdbf5388cf4f47740c76fb58616",
		},
	}

	defaultComputeModuleConfig = map[string]any{
		"defaultTickInterval": "100ms",
		"defaultTimeout":      "300ms",
		"defaultMaxMemoryMBs": int64(64),
	}
)

type deployAndInitializeCapabilitiesRegistryCommand struct{}

func NewDeployAndInitializeCapabilitiesRegistryCommand() *deployAndInitializeCapabilitiesRegistryCommand {
	return &deployAndInitializeCapabilitiesRegistryCommand{}
}

func (c *deployAndInitializeCapabilitiesRegistryCommand) Name() string {
	return "deploy-and-initialize-capabilities-registry"
}

func peerIDToB(peerID string) ([32]byte, error) {
	var peerIDB ragetypes.PeerID
	err := peerIDB.UnmarshalText([]byte(peerID))
	if err != nil {
		return [32]byte{}, err
	}

	return peerIDB, nil
}

func peers(ps []peer) ([][32]byte, error) {
	out := [][32]byte{}
	for _, p := range ps {
		b, err := peerIDToB(p.PeerID)
		if err != nil {
			return nil, err
		}

		out = append(out, b)
	}

	return out, nil
}

func peerToNode(nopID uint32, p peer) (kcr.CapabilitiesRegistryNodeParams, error) {
	peerIDB, err := peerIDToB(p.PeerID)
	if err != nil {
		return kcr.CapabilitiesRegistryNodeParams{}, fmt.Errorf("failed to convert peerID: %w", err)
	}

	sig := strings.TrimPrefix(p.Signer, "0x")
	signerB, err := hex.DecodeString(sig)
	if err != nil {
		return kcr.CapabilitiesRegistryNodeParams{}, fmt.Errorf("failed to convert signer: %w", err)
	}

	keyStr := strings.TrimPrefix(p.EncryptionPublicKey, "0x")
	encKey, err := hex.DecodeString(keyStr)
	if err != nil {
		return kcr.CapabilitiesRegistryNodeParams{}, fmt.Errorf("failed to convert encryptionPublicKey: %w", err)
	}

	var sigb [32]byte
	var encKeyB [32]byte
	copy(sigb[:], signerB)
	copy(encKeyB[:], encKey)

	return kcr.CapabilitiesRegistryNodeParams{
		NodeOperatorId:      nopID,
		P2pId:               peerIDB,
		Signer:              sigb,
		EncryptionPublicKey: encKeyB,
	}, nil
}

// newCapabilityConfig returns a new capability config with the default config set as empty.
// Override the empty default config with functional options.
func newCapabilityConfig(opts ...func(*values.Map)) *capabilitiespb.CapabilityConfig {
	dc := values.EmptyMap()
	for _, opt := range opts {
		opt(dc)
	}

	return &capabilitiespb.CapabilityConfig{
		DefaultConfig: values.ProtoMap(dc),
	}
}

// withDefaultConfig returns a function that sets the default config for a capability by merging
// the provided map with the existing default config.  This is a shallow merge.
func withDefaultConfig(m map[string]any) func(*values.Map) {
	return func(dc *values.Map) {
		overrides, err := values.NewMap(m)
		if err != nil {
			panic(err)
		}
		for k, v := range overrides.Underlying {
			dc.Underlying[k] = v
		}
	}
}

// Run expects the following environment variables to be set:
//
//  1. Deploys the CapabilitiesRegistry contract
//  2. Configures it with a hardcode DON setup, as used by our staging environment.
func (c *deployAndInitializeCapabilitiesRegistryCommand) Run(args []string) {
	ctx := context.Background()

	fs := flag.NewFlagSet(c.Name(), flag.ExitOnError)
	// create flags for all of the env vars then set the env vars to normalize the interface
	// this is a bit of a hack but it's the easiest way to make this work
	ethUrl := fs.String("ethurl", "", "URL of the Ethereum node")
	chainID := fs.Int64("chainid", 11155111, "chain ID of the Ethereum network to deploy to")
	accountKey := fs.String("accountkey", "", "private key of the account to deploy from")
	capabilityRegistryAddress := fs.String("craddress", "", "address of the capability registry")

	err := fs.Parse(args)
	if err != nil ||
		*ethUrl == "" || ethUrl == nil ||
		*chainID == 0 || chainID == nil ||
		*accountKey == "" || accountKey == nil {
		fs.Usage()
		os.Exit(1)
	}

	os.Setenv("ETH_URL", *ethUrl)
	os.Setenv("ETH_CHAIN_ID", fmt.Sprintf("%d", *chainID))
	os.Setenv("ACCOUNT_KEY", *accountKey)

	env := helpers.SetupEnv(false)

	var reg *kcr.CapabilitiesRegistry
	if *capabilityRegistryAddress == "" {
		reg = deployCapabilitiesRegistry(env)
	} else {
		addr := common.HexToAddress(*capabilityRegistryAddress)
		r, innerErr := kcr.NewCapabilitiesRegistry(addr, env.Ec)
		if err != nil {
			panic(innerErr)
		}

		reg = r
	}

	streamsTrigger := kcr.CapabilitiesRegistryCapability{
		LabelledName:   "streams-trigger",
		Version:        "1.0.0",
		CapabilityType: uint8(0), // trigger
	}
	_, err = reg.GetHashedCapabilityId(&bind.CallOpts{}, streamsTrigger.LabelledName, streamsTrigger.Version)
	if err != nil {
		panic(err)
	}

	cronTrigger := kcr.CapabilitiesRegistryCapability{
		LabelledName:   "cron-trigger",
		Version:        "1.0.0",
		CapabilityType: uint8(0), // trigger
	}
	ctid, err := reg.GetHashedCapabilityId(&bind.CallOpts{}, cronTrigger.LabelledName, cronTrigger.Version)
	if err != nil {
		panic(err)
	}

	computeAction := kcr.CapabilitiesRegistryCapability{
		LabelledName:   "custom-compute",
		Version:        "1.0.0",
		CapabilityType: uint8(1), // action
	}
	aid, err := reg.GetHashedCapabilityId(&bind.CallOpts{}, computeAction.LabelledName, computeAction.Version)
	if err != nil {
		panic(err)
	}

	writeChain := kcr.CapabilitiesRegistryCapability{
		LabelledName:   "write_ethereum-testnet-sepolia",
		Version:        "1.0.0",
		CapabilityType: uint8(3), // target
	}
	_, err = reg.GetHashedCapabilityId(&bind.CallOpts{}, writeChain.LabelledName, writeChain.Version)
	if err != nil {
		log.Printf("failed to call GetHashedCapabilityId: %s", err)
	}

	aptosWriteChain := kcr.CapabilitiesRegistryCapability{
		LabelledName:   "write_aptos",
		Version:        "1.0.0",
		CapabilityType: uint8(3), // target
	}
	_, err = reg.GetHashedCapabilityId(&bind.CallOpts{}, aptosWriteChain.LabelledName, aptosWriteChain.Version)
	if err != nil {
		log.Printf("failed to call GetHashedCapabilityId: %s", err)
	}

	ocr := kcr.CapabilitiesRegistryCapability{
		LabelledName:   "offchain_reporting",
		Version:        "1.0.0",
		CapabilityType: uint8(2), // consensus
	}
	ocrid, err := reg.GetHashedCapabilityId(&bind.CallOpts{}, ocr.LabelledName, ocr.Version)
	if err != nil {
		log.Printf("failed to call GetHashedCapabilityId: %s", err)
	}

	tx, err := reg.AddCapabilities(env.Owner, []kcr.CapabilitiesRegistryCapability{
		streamsTrigger,
		writeChain,
		aptosWriteChain,
		ocr,
		cronTrigger,
		computeAction,
	})
	if err != nil {
		log.Printf("failed to call AddCapabilities: %s", err)
	}

	helpers.ConfirmTXMined(ctx, env.Ec, tx, env.ChainID)

	tx, err = reg.AddNodeOperators(env.Owner, []kcr.CapabilitiesRegistryNodeOperator{
		{
			Admin: env.Owner.From,
			Name:  "STAGING_NODE_OPERATOR",
		},
	})
	if err != nil {
		log.Printf("failed to AddNodeOperators: %s", err)
	}

	receipt := helpers.ConfirmTXMined(ctx, env.Ec, tx, env.ChainID)

	recLog, err := reg.ParseNodeOperatorAdded(*receipt.Logs[0])
	if err != nil {
		panic(err)
	}

	nopID := recLog.NodeOperatorId
	nodes := []kcr.CapabilitiesRegistryNodeParams{}
	for _, wfPeer := range workflowDonPeers {
		n, innerErr := peerToNode(nopID, wfPeer)
		if innerErr != nil {
			panic(innerErr)
		}

		n.HashedCapabilityIds = [][32]byte{ocrid, ctid}
		nodes = append(nodes, n)
	}

	tx, err = reg.AddNodes(env.Owner, nodes)
	if err != nil {
		log.Printf("failed to AddNodes: %s", err)
	}

	helpers.ConfirmTXMined(ctx, env.Ec, tx, env.ChainID)

	// workflow DON
	ps, err := peers(workflowDonPeers)
	if err != nil {
		panic(err)
	}

	cc := newCapabilityConfig()
	ccb, err := proto.Marshal(cc)
	if err != nil {
		panic(err)
	}

	computeCfg := newCapabilityConfig(withDefaultConfig(defaultComputeModuleConfig))
	ccfgb, err := proto.Marshal(computeCfg)
	if err != nil {
		panic(err)
	}

	cfgs := []kcr.CapabilitiesRegistryCapabilityConfiguration{
		{
			CapabilityId: ocrid,
			Config:       ccb,
		},
		{
			CapabilityId: ctid,
			Config:       ccb,
		},
		{
			CapabilityId: aid,
			Config:       ccfgb,
		},
	}
	_, err = reg.AddDON(env.Owner, ps, cfgs, true, true, 2)
	if err != nil {
		log.Printf("workflowDON: failed to AddDON: %s", err)
	}

}

func deployCapabilitiesRegistry(env helpers.Environment) *kcr.CapabilitiesRegistry {
	_, tx, contract, err := kcr.DeployCapabilitiesRegistry(env.Owner, env.Ec)
	if err != nil {
		panic(err)
	}

	addr := helpers.ConfirmContractDeployed(context.Background(), env.Ec, tx, env.ChainID)
	fmt.Printf("CapabilitiesRegistry address: %s", addr)
	return contract
}
