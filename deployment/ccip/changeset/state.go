package changeset

import (
	"fmt"

	burn_mint_token_pool "github.com/smartcontractkit/chainlink/v2/core/gethwrappers/ccip/generated/burn_mint_token_pool_1_4_0"
	"github.com/smartcontractkit/chainlink/v2/core/gethwrappers/shared/generated/erc20"

	"github.com/smartcontractkit/chainlink/v2/core/gethwrappers/ccip/generated/mock_usdc_token_messenger"
	"github.com/smartcontractkit/chainlink/v2/core/gethwrappers/ccip/generated/mock_usdc_token_transmitter"
	"github.com/smartcontractkit/chainlink/v2/core/gethwrappers/ccip/generated/usdc_token_pool"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/pkg/errors"

	"github.com/smartcontractkit/chainlink/deployment"
	"github.com/smartcontractkit/chainlink/deployment/ccip/view"
	"github.com/smartcontractkit/chainlink/deployment/ccip/view/v1_0"
	"github.com/smartcontractkit/chainlink/deployment/ccip/view/v1_2"
	"github.com/smartcontractkit/chainlink/deployment/ccip/view/v1_5"
	"github.com/smartcontractkit/chainlink/deployment/ccip/view/v1_6"
	commoncs "github.com/smartcontractkit/chainlink/deployment/common/changeset"
	commontypes "github.com/smartcontractkit/chainlink/deployment/common/types"
	common_v1_0 "github.com/smartcontractkit/chainlink/deployment/common/view/v1_0"
	"github.com/smartcontractkit/chainlink/v2/core/gethwrappers/ccip/generated/ccip_config"
	"github.com/smartcontractkit/chainlink/v2/core/gethwrappers/ccip/generated/commit_store"
	"github.com/smartcontractkit/chainlink/v2/core/gethwrappers/ccip/generated/mock_rmn_contract"
	"github.com/smartcontractkit/chainlink/v2/core/gethwrappers/ccip/generated/registry_module_owner_custom"
	"github.com/smartcontractkit/chainlink/v2/core/gethwrappers/ccip/generated/rmn_home"
	"github.com/smartcontractkit/chainlink/v2/core/gethwrappers/shared/generated/multicall3"

	"github.com/smartcontractkit/chainlink/v2/core/gethwrappers/ccip/generated/ccip_home"
	"github.com/smartcontractkit/chainlink/v2/core/gethwrappers/ccip/generated/fee_quoter"
	"github.com/smartcontractkit/chainlink/v2/core/gethwrappers/ccip/generated/maybe_revert_message_receiver"
	"github.com/smartcontractkit/chainlink/v2/core/gethwrappers/keystone/generated/capabilities_registry"
	"github.com/smartcontractkit/chainlink/v2/core/gethwrappers/shared/generated/burn_mint_erc677"

	"github.com/smartcontractkit/chainlink/v2/core/gethwrappers/ccip/generated/nonce_manager"
	"github.com/smartcontractkit/chainlink/v2/core/gethwrappers/ccip/generated/offramp"
	"github.com/smartcontractkit/chainlink/v2/core/gethwrappers/ccip/generated/onramp"
	"github.com/smartcontractkit/chainlink/v2/core/gethwrappers/ccip/generated/rmn_proxy_contract"
	"github.com/smartcontractkit/chainlink/v2/core/gethwrappers/ccip/generated/rmn_remote"
	"github.com/smartcontractkit/chainlink/v2/core/gethwrappers/ccip/generated/router"
	"github.com/smartcontractkit/chainlink/v2/core/gethwrappers/ccip/generated/token_admin_registry"
	"github.com/smartcontractkit/chainlink/v2/core/gethwrappers/ccip/generated/weth9"
	"github.com/smartcontractkit/chainlink/v2/core/gethwrappers/generated/aggregator_v3_interface"
)

var (
	MockRMN              deployment.ContractType = "MockRMN"
	RMNRemote            deployment.ContractType = "RMNRemote"
	ARMProxy             deployment.ContractType = "ARMProxy"
	WETH9                deployment.ContractType = "WETH9"
	Router               deployment.ContractType = "Router"
	CommitStore          deployment.ContractType = "CommitStore"
	TokenAdminRegistry   deployment.ContractType = "TokenAdminRegistry"
	RegistryModule       deployment.ContractType = "RegistryModuleOwnerCustom"
	NonceManager         deployment.ContractType = "NonceManager"
	FeeQuoter            deployment.ContractType = "FeeQuoter"
	CCIPHome             deployment.ContractType = "CCIPHome"
	CCIPConfig           deployment.ContractType = "CCIPConfig"
	RMNHome              deployment.ContractType = "RMNHome"
	OnRamp               deployment.ContractType = "OnRamp"
	OffRamp              deployment.ContractType = "OffRamp"
	CapabilitiesRegistry deployment.ContractType = "CapabilitiesRegistry"
	PriceFeed            deployment.ContractType = "PriceFeed"
	// Note test router maps to a regular router contract.
	TestRouter          deployment.ContractType = "TestRouter"
	Multicall3          deployment.ContractType = "Multicall3"
	CCIPReceiver        deployment.ContractType = "CCIPReceiver"
	BurnMintToken       deployment.ContractType = "BurnMintToken"
	BurnMintTokenPool   deployment.ContractType = "BurnMintTokenPool"
	USDCToken           deployment.ContractType = "USDCToken"
	USDCMockTransmitter deployment.ContractType = "USDCMockTransmitter"
	USDCTokenMessenger  deployment.ContractType = "USDCTokenMessenger"
	USDCTokenPool       deployment.ContractType = "USDCTokenPool"
)

// CCIPChainState holds a Go binding for all the currently deployed CCIP contracts
// on a chain. If a binding is nil, it means here is no such contract on the chain.
type CCIPChainState struct {
	commoncs.MCMSWithTimelockState
	commoncs.LinkTokenState
	commoncs.StaticLinkTokenState
	OnRamp    *onramp.OnRamp
	OffRamp   *offramp.OffRamp
	FeeQuoter *fee_quoter.FeeQuoter
	// We need 2 RMNProxy contracts because we are in the process of migrating to a new version.
	// We will switch to the existing one once the migration is complete.
	// This is the new RMNProxy contract that will be used for testing RMNRemote before migration.
	// Initially RMNProxyNew will point to RMNRemote
	RMNProxyNew *rmn_proxy_contract.RMNProxyContract
	// Existing RMNProxy contract that is used in production, This already has existing 1.5 RMN set.
	// once RMNRemote is tested with RMNProxyNew, as part of migration
	// RMNProxyExisting will point to RMNRemote. This will switch over CCIP 1.5 to 1.6
	RMNProxyExisting   *rmn_proxy_contract.RMNProxyContract
	NonceManager       *nonce_manager.NonceManager
	TokenAdminRegistry *token_admin_registry.TokenAdminRegistry
	RegistryModule     *registry_module_owner_custom.RegistryModuleOwnerCustom
	Router             *router.Router
	CommitStore        *commit_store.CommitStore
	Weth9              *weth9.WETH9
	RMNRemote          *rmn_remote.RMNRemote
	MockRMN            *mock_rmn_contract.MockRMNContract
	// Map between token Descriptor (e.g. LinkSymbol, WethSymbol)
	// and the respective token contract
	// This is more of an illustration of how we'll have tokens, and it might need some work later to work properly.
	// Not all tokens will be burn and mint tokens.
	BurnMintTokens677  map[TokenSymbol]*burn_mint_erc677.BurnMintERC677
	BurnMintTokenPools map[TokenSymbol]*burn_mint_token_pool.BurnMintTokenPool
	// Map between token Symbol (e.g. LinkSymbol, WethSymbol)
	// and the respective aggregator USD feed contract
	USDFeeds map[TokenSymbol]*aggregator_v3_interface.AggregatorV3Interface

	// Note we only expect one of these (on the home chain)
	CapabilityRegistry *capabilities_registry.CapabilitiesRegistry
	CCIPHome           *ccip_home.CCIPHome
	RMNHome            *rmn_home.RMNHome
	// TODO remove once staging upgraded.
	CCIPConfig *ccip_config.CCIPConfig

	// Test contracts
	Receiver               *maybe_revert_message_receiver.MaybeRevertMessageReceiver
	TestRouter             *router.Router
	USDCTokenPool          *usdc_token_pool.USDCTokenPool
	MockUSDCTransmitter    *mock_usdc_token_transmitter.MockE2EUSDCTransmitter
	MockUSDCTokenMessenger *mock_usdc_token_messenger.MockE2EUSDCTokenMessenger
	Multicall3             *multicall3.Multicall3
}

func (c CCIPChainState) GenerateView() (view.ChainView, error) {
	chainView := view.NewChain()
	if c.Router != nil {
		routerView, err := v1_2.GenerateRouterView(c.Router)
		if err != nil {
			return chainView, errors.Wrapf(err, "failed to generate router view for router %s", c.Router.Address().String())
		}
		chainView.Router[c.Router.Address().Hex()] = routerView
	}
	if c.TokenAdminRegistry != nil {
		taView, err := v1_5.GenerateTokenAdminRegistryView(c.TokenAdminRegistry)
		if err != nil {
			return chainView, errors.Wrapf(err, "failed to generate token admin registry view for token admin registry %s", c.TokenAdminRegistry.Address().String())
		}
		chainView.TokenAdminRegistry[c.TokenAdminRegistry.Address().Hex()] = taView
	}
	if c.NonceManager != nil {
		nmView, err := v1_6.GenerateNonceManagerView(c.NonceManager)
		if err != nil {
			return chainView, errors.Wrapf(err, "failed to generate nonce manager view for nonce manager %s", c.NonceManager.Address().String())
		}
		chainView.NonceManager[c.NonceManager.Address().Hex()] = nmView
	}
	if c.RMNRemote != nil {
		rmnView, err := v1_6.GenerateRMNRemoteView(c.RMNRemote)
		if err != nil {
			return chainView, errors.Wrapf(err, "failed to generate rmn remote view for rmn remote %s", c.RMNRemote.Address().String())
		}
		chainView.RMN[c.RMNRemote.Address().Hex()] = rmnView
	}
	if c.FeeQuoter != nil && c.Router != nil && c.TokenAdminRegistry != nil {
		fqView, err := v1_6.GenerateFeeQuoterView(c.FeeQuoter, c.Router, c.TokenAdminRegistry)
		if err != nil {
			return chainView, errors.Wrapf(err, "failed to generate fee quoter view for fee quoter %s", c.FeeQuoter.Address().String())
		}
		chainView.FeeQuoter[c.FeeQuoter.Address().Hex()] = fqView
	}

	if c.OnRamp != nil && c.Router != nil && c.TokenAdminRegistry != nil {
		onRampView, err := v1_6.GenerateOnRampView(
			c.OnRamp,
			c.Router,
			c.TokenAdminRegistry,
		)
		if err != nil {
			return chainView, errors.Wrapf(err, "failed to generate on ramp view for on ramp %s", c.OnRamp.Address().String())
		}
		chainView.OnRamp[c.OnRamp.Address().Hex()] = onRampView
	}

	if c.OffRamp != nil && c.Router != nil {
		offRampView, err := v1_6.GenerateOffRampView(
			c.OffRamp,
			c.Router,
		)
		if err != nil {
			return chainView, errors.Wrapf(err, "failed to generate off ramp view for off ramp %s", c.OffRamp.Address().String())
		}
		chainView.OffRamp[c.OffRamp.Address().Hex()] = offRampView
	}

	if c.CommitStore != nil {
		commitStoreView, err := v1_5.GenerateCommitStoreView(c.CommitStore)
		if err != nil {
			return chainView, errors.Wrapf(err, "failed to generate commit store view for commit store %s", c.CommitStore.Address().String())
		}
		chainView.CommitStore[c.CommitStore.Address().Hex()] = commitStoreView
	}

	if c.RMNProxyNew != nil {
		rmnProxyView, err := v1_0.GenerateRMNProxyView(c.RMNProxyNew)
		if err != nil {
			return chainView, errors.Wrapf(err, "failed to generate rmn proxy view for rmn proxy %s", c.RMNProxyNew.Address().String())
		}
		chainView.RMNProxy[c.RMNProxyNew.Address().Hex()] = rmnProxyView
	}
	if c.CapabilityRegistry != nil {
		capRegView, err := common_v1_0.GenerateCapabilityRegistryView(c.CapabilityRegistry)
		if err != nil {
			return chainView, errors.Wrapf(err, "failed to generate capability registry view for capability registry %s", c.CapabilityRegistry.Address().String())
		}
		chainView.CapabilityRegistry[c.CapabilityRegistry.Address().Hex()] = capRegView
	}
	if c.MCMSWithTimelockState.Timelock != nil {
		mcmsView, err := c.MCMSWithTimelockState.GenerateMCMSWithTimelockView()
		if err != nil {
			return chainView, errors.Wrapf(err, "failed to generate MCMS with timelock view for MCMS with timelock %s", c.MCMSWithTimelockState.Timelock.Address().String())
		}
		chainView.MCMSWithTimelock = mcmsView
	}
	if c.LinkToken != nil {
		linkTokenView, err := c.GenerateLinkView()
		if err != nil {
			return chainView, errors.Wrapf(err, "failed to generate link token view for link token %s", c.LinkToken.Address().String())
		}
		chainView.LinkToken = linkTokenView
	}
	if c.StaticLinkToken != nil {
		staticLinkTokenView, err := c.GenerateStaticLinkView()
		if err != nil {
			return chainView, err
		}
		chainView.StaticLinkToken = staticLinkTokenView
	}
	return chainView, nil
}

// CCIPOnChainState state always derivable from an address book.
// Offchain state always derivable from a list of nodeIds.
// Note can translate this into Go struct needed for MCMS/Docs/UI.
type CCIPOnChainState struct {
	// Populated go bindings for the appropriate version for all contracts.
	// We would hold 2 versions of each contract here. Once we upgrade we can phase out the old one.
	// When generating bindings, make sure the package name corresponds to the version.
	Chains map[uint64]CCIPChainState
}

func (s CCIPOnChainState) View(chains []uint64) (map[string]view.ChainView, error) {
	m := make(map[string]view.ChainView)
	for _, chainSelector := range chains {
		chainInfo, err := deployment.ChainInfo(chainSelector)
		if err != nil {
			return m, err
		}
		if _, ok := s.Chains[chainSelector]; !ok {
			return m, fmt.Errorf("chain not supported %d", chainSelector)
		}
		chainState := s.Chains[chainSelector]
		chainView, err := chainState.GenerateView()
		if err != nil {
			return m, err
		}
		m[chainInfo.ChainName] = chainView
	}
	return m, nil
}

func LoadOnchainState(e deployment.Environment) (CCIPOnChainState, error) {
	state := CCIPOnChainState{
		Chains: make(map[uint64]CCIPChainState),
	}
	for chainSelector, chain := range e.Chains {
		addresses, err := e.ExistingAddresses.AddressesForChain(chainSelector)
		if err != nil {
			// Chain not found in address book, initialize empty
			if errors.Is(err, deployment.ErrChainNotFound) {
				addresses = make(map[string]deployment.TypeAndVersion)
			} else {
				return state, err
			}
		}
		chainState, err := LoadChainState(chain, addresses)
		if err != nil {
			return state, err
		}
		state.Chains[chainSelector] = chainState
	}
	return state, nil
}

// LoadChainState Loads all state for a chain into state
func LoadChainState(chain deployment.Chain, addresses map[string]deployment.TypeAndVersion) (CCIPChainState, error) {
	var state CCIPChainState
	mcmsWithTimelock, err := commoncs.MaybeLoadMCMSWithTimelockState(chain, addresses)
	if err != nil {
		return state, err
	}
	state.MCMSWithTimelockState = *mcmsWithTimelock

	linkState, err := commoncs.MaybeLoadLinkTokenState(chain, addresses)
	if err != nil {
		return state, err
	}
	state.LinkTokenState = *linkState
	staticLinkState, err := commoncs.MaybeLoadStaticLinkTokenState(chain, addresses)
	if err != nil {
		return state, err
	}
	state.StaticLinkTokenState = *staticLinkState
	for address, tvStr := range addresses {
		switch tvStr.String() {
		case deployment.NewTypeAndVersion(commontypes.RBACTimelock, deployment.Version1_0_0).String(),
			deployment.NewTypeAndVersion(commontypes.ProposerManyChainMultisig, deployment.Version1_0_0).String(),
			deployment.NewTypeAndVersion(commontypes.CancellerManyChainMultisig, deployment.Version1_0_0).String(),
			deployment.NewTypeAndVersion(commontypes.BypasserManyChainMultisig, deployment.Version1_0_0).String(),
			deployment.NewTypeAndVersion(commontypes.LinkToken, deployment.Version1_0_0).String(),
			deployment.NewTypeAndVersion(commontypes.StaticLinkToken, deployment.Version1_0_0).String():
			// Skip common contracts, they are already loaded.
			continue
		case deployment.NewTypeAndVersion(CapabilitiesRegistry, deployment.Version1_0_0).String():
			cr, err := capabilities_registry.NewCapabilitiesRegistry(common.HexToAddress(address), chain.Client)
			if err != nil {
				return state, err
			}
			state.CapabilityRegistry = cr
		case deployment.NewTypeAndVersion(OnRamp, deployment.Version1_6_0_dev).String():
			onRampC, err := onramp.NewOnRamp(common.HexToAddress(address), chain.Client)
			if err != nil {
				return state, err
			}
			state.OnRamp = onRampC
		case deployment.NewTypeAndVersion(OffRamp, deployment.Version1_6_0_dev).String():
			offRamp, err := offramp.NewOffRamp(common.HexToAddress(address), chain.Client)
			if err != nil {
				return state, err
			}
			state.OffRamp = offRamp
		case deployment.NewTypeAndVersion(ARMProxy, deployment.Version1_0_0).String():
			armProxy, err := rmn_proxy_contract.NewRMNProxyContract(common.HexToAddress(address), chain.Client)
			if err != nil {
				return state, err
			}
			state.RMNProxyExisting = armProxy
		case deployment.NewTypeAndVersion(ARMProxy, deployment.Version1_6_0_dev).String():
			armProxy, err := rmn_proxy_contract.NewRMNProxyContract(common.HexToAddress(address), chain.Client)
			if err != nil {
				return state, err
			}
			state.RMNProxyNew = armProxy
		case deployment.NewTypeAndVersion(ARMProxy, deployment.Version1_6_0_dev).String():
			armProxy, err := rmn_proxy_contract.NewRMNProxyContract(common.HexToAddress(address), chain.Client)
			if err != nil {
				return state, err
			}
			state.RMNProxyNew = armProxy
		case deployment.NewTypeAndVersion(MockRMN, deployment.Version1_0_0).String():
			mockRMN, err := mock_rmn_contract.NewMockRMNContract(common.HexToAddress(address), chain.Client)
			if err != nil {
				return state, err
			}
			state.MockRMN = mockRMN
		case deployment.NewTypeAndVersion(RMNRemote, deployment.Version1_6_0_dev).String():
			rmnRemote, err := rmn_remote.NewRMNRemote(common.HexToAddress(address), chain.Client)
			if err != nil {
				return state, err
			}
			state.RMNRemote = rmnRemote
		case deployment.NewTypeAndVersion(RMNHome, deployment.Version1_6_0_dev).String():
			rmnHome, err := rmn_home.NewRMNHome(common.HexToAddress(address), chain.Client)
			if err != nil {
				return state, err
			}
			state.RMNHome = rmnHome
		case deployment.NewTypeAndVersion(WETH9, deployment.Version1_0_0).String():
			weth9, err := weth9.NewWETH9(common.HexToAddress(address), chain.Client)
			if err != nil {
				return state, err
			}
			state.Weth9 = weth9
		case deployment.NewTypeAndVersion(NonceManager, deployment.Version1_6_0_dev).String():
			nm, err := nonce_manager.NewNonceManager(common.HexToAddress(address), chain.Client)
			if err != nil {
				return state, err
			}
			state.NonceManager = nm
		case deployment.NewTypeAndVersion(CommitStore, deployment.Version1_5_0).String():
			cs, err := commit_store.NewCommitStore(common.HexToAddress(address), chain.Client)
			if err != nil {
				return state, err
			}
			state.CommitStore = cs
		case deployment.NewTypeAndVersion(TokenAdminRegistry, deployment.Version1_5_0).String():
			tm, err := token_admin_registry.NewTokenAdminRegistry(common.HexToAddress(address), chain.Client)
			if err != nil {
				return state, err
			}
			state.TokenAdminRegistry = tm
		case deployment.NewTypeAndVersion(RegistryModule, deployment.Version1_5_0).String():
			rm, err := registry_module_owner_custom.NewRegistryModuleOwnerCustom(common.HexToAddress(address), chain.Client)
			if err != nil {
				return state, err
			}
			state.RegistryModule = rm
		case deployment.NewTypeAndVersion(Router, deployment.Version1_2_0).String():
			r, err := router.NewRouter(common.HexToAddress(address), chain.Client)
			if err != nil {
				return state, err
			}
			state.Router = r
		case deployment.NewTypeAndVersion(TestRouter, deployment.Version1_2_0).String():
			r, err := router.NewRouter(common.HexToAddress(address), chain.Client)
			if err != nil {
				return state, err
			}
			state.TestRouter = r
		case deployment.NewTypeAndVersion(FeeQuoter, deployment.Version1_6_0_dev).String():
			fq, err := fee_quoter.NewFeeQuoter(common.HexToAddress(address), chain.Client)
			if err != nil {
				return state, err
			}
			state.FeeQuoter = fq
		case deployment.NewTypeAndVersion(USDCToken, deployment.Version1_0_0).String():
			ut, err := burn_mint_erc677.NewBurnMintERC677(common.HexToAddress(address), chain.Client)
			if err != nil {
				return state, err
			}
			state.BurnMintTokens677 = map[TokenSymbol]*burn_mint_erc677.BurnMintERC677{
				USDCSymbol: ut,
			}
		case deployment.NewTypeAndVersion(USDCTokenPool, deployment.Version1_0_0).String():
			utp, err := usdc_token_pool.NewUSDCTokenPool(common.HexToAddress(address), chain.Client)
			if err != nil {
				return state, err
			}
			state.USDCTokenPool = utp
		case deployment.NewTypeAndVersion(USDCMockTransmitter, deployment.Version1_0_0).String():
			umt, err := mock_usdc_token_transmitter.NewMockE2EUSDCTransmitter(common.HexToAddress(address), chain.Client)
			if err != nil {
				return state, err
			}
			state.MockUSDCTransmitter = umt
		case deployment.NewTypeAndVersion(USDCTokenMessenger, deployment.Version1_0_0).String():
			utm, err := mock_usdc_token_messenger.NewMockE2EUSDCTokenMessenger(common.HexToAddress(address), chain.Client)
			if err != nil {
				return state, err
			}
			state.MockUSDCTokenMessenger = utm
		case deployment.NewTypeAndVersion(CCIPHome, deployment.Version1_6_0_dev).String():
			ccipHome, err := ccip_home.NewCCIPHome(common.HexToAddress(address), chain.Client)
			if err != nil {
				return state, err
			}
			state.CCIPHome = ccipHome
		case deployment.NewTypeAndVersion(CCIPConfig, deployment.Version1_0_0).String():
			// TODO: Remove once staging upgraded.
			ccipConfig, err := ccip_config.NewCCIPConfig(common.HexToAddress(address), chain.Client)
			if err != nil {
				return state, err
			}
			state.CCIPConfig = ccipConfig
		case deployment.NewTypeAndVersion(CCIPReceiver, deployment.Version1_0_0).String():
			mr, err := maybe_revert_message_receiver.NewMaybeRevertMessageReceiver(common.HexToAddress(address), chain.Client)
			if err != nil {
				return state, err
			}
			state.Receiver = mr
		case deployment.NewTypeAndVersion(Multicall3, deployment.Version1_0_0).String():
			mc, err := multicall3.NewMulticall3(common.HexToAddress(address), chain.Client)
			if err != nil {
				return state, err
			}
			state.Multicall3 = mc
		case deployment.NewTypeAndVersion(PriceFeed, deployment.Version1_0_0).String():
			feed, err := aggregator_v3_interface.NewAggregatorV3Interface(common.HexToAddress(address), chain.Client)
			if err != nil {
				return state, err
			}
			if state.USDFeeds == nil {
				state.USDFeeds = make(map[TokenSymbol]*aggregator_v3_interface.AggregatorV3Interface)
			}
			desc, err := feed.Description(&bind.CallOpts{})
			if err != nil {
				return state, err
			}
			key, ok := MockDescriptionToTokenSymbol[desc]
			if !ok {
				return state, fmt.Errorf("unknown feed description %s", desc)
			}
			state.USDFeeds[key] = feed
		case deployment.NewTypeAndVersion(BurnMintTokenPool, deployment.Version1_5_1).String():
			pool, err := burn_mint_token_pool.NewBurnMintTokenPool(common.HexToAddress(address), chain.Client)
			if err != nil {
				return state, err
			}
			if state.BurnMintTokenPools == nil {
				state.BurnMintTokenPools = make(map[TokenSymbol]*burn_mint_token_pool.BurnMintTokenPool)
			}
			tokAddress, err := pool.GetToken(nil)
			if err != nil {
				return state, err
			}
			tok, err := erc20.NewERC20(tokAddress, chain.Client)
			if err != nil {
				return state, err
			}
			symbol, err := tok.Symbol(nil)
			if err != nil {
				return state, err
			}
			state.BurnMintTokenPools[TokenSymbol(symbol)] = pool
		case deployment.NewTypeAndVersion(BurnMintToken, deployment.Version1_0_0).String():
			tok, err := burn_mint_erc677.NewBurnMintERC677(common.HexToAddress(address), chain.Client)
			if err != nil {
				return state, err
			}
			if state.BurnMintTokens677 == nil {
				state.BurnMintTokens677 = make(map[TokenSymbol]*burn_mint_erc677.BurnMintERC677)
			}
			symbol, err := tok.Symbol(nil)
			if err != nil {
				return state, fmt.Errorf("failed to get token symbol of token at %s: %w", address, err)
			}
			state.BurnMintTokens677[TokenSymbol(symbol)] = tok
		default:
			return state, fmt.Errorf("unknown contract %s", tvStr)
		}
	}
	return state, nil
}
