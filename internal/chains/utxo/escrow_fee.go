package utxo

const (
	txVersionAndLockTimeSize    = 8
	outpointSize                = 36
	sequenceSize                = 4
	p2pkhOutputSize             = 34
	compressedPubKeySize        = 33
	pushOpcodeSize              = 1
	witnessMarkerFlagSize       = 2
	p2wshRelaySafetyVBytes      = 10
	p2shRelaySafetyBytes        = 20
	p2shSchnorrRelaySafetyBytes = 80
)

// EstimateP2WSHMultisigSpendVSize estimates the virtual size of a native
// segwit multisig spend with nInputs escrow inputs and nOuts P2PKH-style outputs.
func EstimateP2WSHMultisigSpendVSize(nInputs int, threshold int, nOuts int) int {
	nInputs = positiveOrOne(nInputs)
	nOuts = positiveOrOne(nOuts)
	redeemScriptSize := multisigRedeemScriptSize(threshold)
	witnessPerInput := varIntSerializeSize(uint64(threshold+2)) +
		1 +
		threshold*(varIntSerializeSize(maxECDSASigWithHashTypeSize)+maxECDSASigWithHashTypeSize) +
		varIntSerializeSize(uint64(redeemScriptSize)) + redeemScriptSize
	inputBaseSize := outpointSize + varIntSerializeSize(0) + sequenceSize
	baseSize := txVersionAndLockTimeSize +
		varIntSerializeSize(uint64(nInputs)) +
		inputBaseSize*nInputs +
		varIntSerializeSize(uint64(nOuts)) +
		p2pkhOutputSize*nOuts

	return weightToVSize(baseSize*4 + witnessMarkerFlagSize + witnessPerInput*nInputs)
}

// EstimateP2WSHMultisigSpendRelayVSize adds a small relay-policy allowance on
// top of the structural P2WSH estimate. The extra bytes cover real-world
// serialization variance such as destination script type and DER signature
// length so fee locks do not sit just below min relay fee.
func EstimateP2WSHMultisigSpendRelayVSize(nInputs int, threshold int, nOuts int) int {
	return EstimateP2WSHMultisigSpendVSize(nInputs, threshold, nOuts) + p2wshRelaySafetyVBytes
}

// EstimateP2SHMultisigSpendSize estimates the serialized size of a legacy P2SH
// multisig spend with nInputs escrow inputs and nOuts P2PKH-style outputs.
func EstimateP2SHMultisigSpendSize(nInputs int, threshold int, nOuts int, sigWithHashTypeSize int, extraBytes int) int {
	nInputs = positiveOrOne(nInputs)
	nOuts = positiveOrOne(nOuts)
	redeemScriptSize := multisigRedeemScriptSize(threshold)
	scriptSigSize := 1 +
		threshold*(pushOpcodeSize+sigWithHashTypeSize) +
		(pushOpcodeSize + redeemScriptSize)
	inputSize := outpointSize + varIntSerializeSize(uint64(scriptSigSize)) + scriptSigSize + sequenceSize

	return txVersionAndLockTimeSize +
		varIntSerializeSize(uint64(nInputs)) +
		inputSize*nInputs +
		varIntSerializeSize(uint64(nOuts)) +
		p2pkhOutputSize*nOuts +
		extraBytes
}

const (
	maxECDSASigWithHashTypeSize = 73
	schnorrSigWithHashTypeSize  = 65
)

func EstimateP2SHECDSAMultisigSpendSize(nInputs int, threshold int, nOuts int, extraBytes int) int {
	return EstimateP2SHMultisigSpendSize(nInputs, threshold, nOuts, maxECDSASigWithHashTypeSize, extraBytes)
}

func EstimateP2SHECDSAMultisigSpendRelaySize(nInputs int, threshold int, nOuts int) int {
	return EstimateP2SHECDSAMultisigSpendSize(nInputs, threshold, nOuts, p2shRelaySafetyBytes*positiveOrOne(nInputs))
}

func EstimateP2SHSchnorrMultisigSpendSize(nInputs int, threshold int, nOuts int, extraBytes int) int {
	return EstimateP2SHMultisigSpendSize(nInputs, threshold, nOuts, schnorrSigWithHashTypeSize, extraBytes)
}

func EstimateP2SHSchnorrMultisigSpendRelaySize(nInputs int, threshold int, nOuts int) int {
	return EstimateP2SHSchnorrMultisigSpendSize(nInputs, threshold, nOuts, p2shSchnorrRelaySafetyBytes*positiveOrOne(nInputs))
}

func multisigRedeemScriptSize(threshold int) int {
	return 3 + (threshold+1)*(pushOpcodeSize+compressedPubKeySize)
}

func weightToVSize(weight int) int {
	return (weight + 3) / 4
}

func varIntSerializeSize(val uint64) int {
	switch {
	case val < 0xfd:
		return 1
	case val <= 0xffff:
		return 3
	case val <= 0xffffffff:
		return 5
	default:
		return 9
	}
}

func positiveOrOne(v int) int {
	if v < 1 {
		return 1
	}
	return v
}
