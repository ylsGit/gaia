# FaQ

## What is the Blockchain?

The blockchain is a technology that enables the development of permissionless decentralized applications. It does so by providing a method to securely replicate a ledger among parties that do not necessarily trust each other. In other words, blockchains allow the creation of a new kind of digital application that do not require a central coordinator to operate. Think electronic money without central banks, digital asset trading without centrally-operated stock exchanges, or social network platforms without central administrators. 

## What is Cosmos?

Cosmos is a network of interconnected blockchains.  Cosmosa also designates the set of tools and protocols that facilitate the development, deployment and interconnection of blockchains.

> Read about Cosmos https://cosmos.network/intro

## What is the Cosmos Hub?

The Cosmos Hub is the first blockchain in the Cosmos Network. The goal of the Cosmos Hub is to facilitate the connection between the multitude of blockchains that will eventually comprise the Cosmos Network. As it will connect to many blockchains, and strive to have high security, the Cosmos Hub will also be in prime position to become one of the biggest decentralized custodian for digitial assets. 

## What is the ATOM token?

The ATOM token is the main cryptoasset of the Cosmos Hub blockchain. If you hold ATOMs, you can temporarily lock them up in order to contribute to the security of the Cosmos Hub via a mechanism called [Staking](#what-is-staking). In exchange for locking them up, you will receive [rewards](#what-rewards-can-i-expect-when-i-stake-my-ATOMs?) newly minted ATOMs as well a share of the transaction fees collected by the blockchain. However, do note that staking [is not risk free](#what-are-the-risks-associated-with-staking?). 

## How to get ATOMs?

ATOMs can be obtained on a number of cryptocurrency exchanges. A  list of such exchanges can be found on this [third party website](https://messari.io/asset/cosmos/exchanges). 

## What is Staking?

Staking is the process of locking up a digital asset (ATOM in the case of the Cosmso Hub) in order to provide economic security for a public blockchain. Public blockchains are permissionless networks, meaning anyone is free to participate in maintaining them. As a result, it is possible for some of the maintainers of the network (called [validators](#what-is-a-validator-?)) to act maliciously. In order to incentivise maintainers to behave in the best interest of the network, the locked up assets are at risk of being partially slashed (i.e. destroyed) if evidence is brought that a fault was committed. For more, see [Risks].

Note that due to practical constraints of the software, the number of validators on the Cosmos Hub has to be capped (currently at 125). This does not mean that ATOM holders that do not operate validators themselves can't participate in securing the network. In fact, ATOMs are designed to let each holder participate in securing the network via a mechanism called [delegation](#what-is-delegating?). When ATOM holders stake their ATOMs, they must choose one or more validator to delegate to. They will then be eligible for receiving rewards, but also be at [risk of slashing] if the validator(s) they chose misbehave. 

## What is the ATOM Staking Process?

Staking ATOM is relatively straightforward. Users need to:

1. Obtain ATOMs. See [How can I get ATOMs?](anchor) for more.
3. [Delegate](#what-is-delegating?) their ATOMs to one or more [validators] of their choice using a [Wallet]. 

And that's it! 

## What is a Validator? 

Validators are special actors in the network responsible for adding new blocks of transactions to the blockchain. Anyone can declare themselves as validator candidate, but only 125 candidates can enter the set of active validators. Validator candidates are chosen based on the amount of voting power associated with their operator account. Voting power is obtained when ATOM holders start staking, in proportion of the amount of ATOMs staked. This voting power must then be [delegated](#what-is-delegating?) to one or multiple validator candidates. As a result, ATOM holders will have to choose validator(s) when they stake. 

In order to add blocks to the blockchain, validators electronically sign block proposals that are valid according to the protocol (the proposer of a given block is selected among validators according to a specific algorithm, and rotates every block). Each signature is weighted by the voting power of the validator, and a block is considered valid if it is signed twice by more than two thirds of validators, weighted by their respective voting power. 

## What is delegating?

Blockchains that use staking, such as the Cosmos Hub, are primarely operated by a set of actors called [validators](what-is-a-validator?). The status of a validator (active or candidate), as well as its weight is established based on its amount of voting power. Voting power is obtained when ATOMs are locked up in the process of staking, and must be granted to a validator or validator candidate. This is called delegating. 

In practice, delegating is done automatically when ATOM holders start staking. A single staking transaction needs to be sent for the whole proces of locking up ATOMs and delegating to validator(s) via a [wallet]. Users will generally only be asked for the amount of ATOMs to stake and the validator(s) they want to delegate to. The Wallet will take care of the rest (i.e. generating and sending the transaction).

Note that validators never obtain custody of the ATOMs delegated to them. There is no risk for validators to "steal" their delegators ATOM. However, there is a risk for delegated ATOMs to be slashed should the validator they are delegated to misbehave. See [What are the risks associated with Staking] for more. 

## What happens to ATOMs when they are staked?

When users stake ATOMs, they effectively lock them up for an indefinite period of time. This means ATOMs cannot be transferred anymore, which is guaranteed by the protocol itself. However, users are free to trigger the process to unlock their ATOMs at any point after they started staking (via the Wallet of their choice). This is called "undelegating". It will take 21 days for ATOMs to be transferrable again after the transaction to undelegate has been sent. 

## How should ATOM holders choose the Validator(s) to stake with? 

In order to choose validator(s), ATOM holders have access to a range of information directly in [Lunie](https://lunie.io) or other [Cosmos block explorers]():

- **Validator's moniker**: Name of the validator candidate.
- **Validator's description**: Description provided by the validator operator.
- **Validator's website**: Link to the validator's website.
- **Initial commission rate**: The [commission](#what-is-a-validator's-commission) rate on rewards charged to any delegator by the validator. 
- **Commission max change rate:** The maximum daily increase of the validator's commission. This parameter cannot be changed by the validator operator. 
- **Maximum commission:** The maximum commission rate this validator candidate can charge. This parameter cannot be changed by the validator operator. 
- **Minimum self-bond amount**: Minimum amount of ATOMs the validator candidate need to have bonded at all time. If the validator's self-bonded stake falls below this limit, their entire staking pool (i.e. all its delegators) will unbond. This parameter exists as a safeguard for delegators. Indeed, when a validator misbehaves, part of their total stake gets slashed. This included the validator's self-delegateds stake as well as their delegators' stake. Thus, a validator with a high amount of self-delegated ATOMs has more skin-in-the-game than a validator with a low amount. The minimum self-bond amount parameter guarantees to delegators that a validator will never fall below a certain amount of self-bonded stake, thereby ensuring a minimum level of skin-in-the-game. This parameter can only be increased by the validator operator. 

Beyond these on-chain information, delegators are encouraged to visit validators' respective websites in order to learn more about their operation and security practices. 

## What is a Validator's commission

In the process of staking, rewards are generated proportional to the amount of ATOMs staked. Of these rewards, a certain percentage goes to the validator to which the ATOMs are delegated. This percentage is called the Commission, and is set by validator operator themselves. 

For example, if an ATOM holder delegates all their ATOMs to a single validator with a commission of 10%, then 10% of this holder's rewards will go to that validator. Commission is therefore an important parameter for ATOM holders to take into account in deciding which validator(s) to delegate to.  Note that some validators may apply higher commission because they operate a more complex setup, meaning they incur higher operating costs (see Risks). It is the responsibility of the delegator to assess the commission of validators they delegate to with regards to the service they offer. 

## What rewards can be expected when staking ATOMs?

Staking rewards come from two different sources:

- **ATOM inflation**: The total supply of ATOM is inflated each block to reward delegators. The inflation rate is a global parameter calculated based on the percentage of ATOM staked.
- **Transaction fees**: Each transaction processed by the network comes with transaction fees, that can be paid in multiple token denominations. These transaction fees are collected by the network and distributed to each delegator proportional to their stake. 

Staking rewards are automatically collected, but they must be actively withdrawn by delegators by sending of a transaction in order to become available. This can be done via [Wallets]. Note that the [validator commission] is deduced from rewards before they are distributed.

## What are the risks associated with Staking?

Staking ATOMs is not risk-free. ATOMs delegated to a validator can be partially slashed (i.e. forfeited without possibility of recovery) should the validator misbehave. On the Cosmos Hub, there are currently two attributable faults that can lead to a slashing event:

- If the validator is offline for too long (missed 500 of the last 10.000 blocks), the ATOMs delegated to them will be slashed by 0.01%. As a delegator, it is important to delegate to validators with good uptime to minimize the risk of being slashed from this fault. 
- If the validator signs two different blocks at the same height, the ATOMs delegated to them will be slashed by 5%. This fault is harder to anticipate, as it can result from bad operation practices or outright malicious intent from the validator operator. Delegators should make sure that the validators in order to prevent slashing from this fault. 

## What is Governance? 

Governance refers to the ability for stakers to vote on proposals that affect the evolution of the network. Proposals can be submitted by any ATOM holder, but only proposals that come with a sufficient deposit (current minimum deposit is 512 ATOMs) are eligible to be voted on. Deposit can be crowdfunded and need not be entirely provided by the user submitting the proposal. 

Once a proposal gets sufficient deposit, it enters the voting period, which lasts for 14 days. Most block explorers show the currently active proposals. ATOM stakers can vote on such proposals, with a voting power proportional to their amount of staked ATOMs. If they do not vote, they inherit the vote from the validator(s) they delegated to. Voting can be performed via most [Wallets].

## What is IBC?

IBC is an acronym for Inter-Blockchain Communication Protocol, which is a protocol for secure message passing between heterogeneous blockchains. It is a central piece of the Cosmos vision, as it will enable blockchains to interract in ways that were not possible before. For a deeper look at IBC, please refer to the [IBC documentation](https://github.com/cosmos/ics/tree/master/ibc). 

## What will IBC enable on the Cosmos Hub?

Cosmos Hub will be one of the first blockchain to implement IBC. While any feature must be approved by the Hub's governance in order to be adopted, it is likely that IBC will enable many new features such as:

- Inter-blockchain token transfers: From their Cosmos Hub wallet, users will be able to hold, send and receive tokens originating from other blockchains. The number of available tokens will grow as the number of IBC connexions with the Cosmos Hub increases.
- Interchain Staking: IBC will enable Cosmos Hub validators to provide security for other chains. In practice, it means that ATOMs will be able to secure both the Cosmos Hub and other chains. Validators will be able to select the chains they validate, and their delegators will share the resulting risks and rewards. 

These two features are excepted to be implemented early, but many others will likely see the light of day. 

