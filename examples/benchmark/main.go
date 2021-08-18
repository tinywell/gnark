package main

import (
	"fmt"
	"math/big"
	"os"
	"strconv"
	"time"

	"github.com/consensys/gnark-crypto/ecc"
	bls12381fr "github.com/consensys/gnark-crypto/ecc/bls12-381/fr"
	bn254fr "github.com/consensys/gnark-crypto/ecc/bn254/fr"
	"github.com/consensys/gnark-crypto/ecc/bn254/fr/kzg"
	"github.com/consensys/gnark/backend"
	"github.com/consensys/gnark/backend/plonk"
	"github.com/consensys/gnark/frontend"
	"github.com/consensys/gnark/internal/backend/bn254/cs"
	cs_bn254 "github.com/consensys/gnark/internal/backend/bn254/cs"
	witness_bn254 "github.com/consensys/gnark/internal/backend/bn254/witness"
	"github.com/pkg/profile"
)

func main() {
	if len(os.Args) != 2 {
		fmt.Println("usage is ./benchmark [nbConstraints]")
		os.Exit(-1)
	}
	n, err := strconv.Atoi(os.Args[1])
	if err != nil {
		panic(err)
	}

	// write to stdout
	const curveID = ecc.BN254
	// generate dummy circuit and witness
	// _, ccs, _ := generateCircuit(n, curveID)
	var circuit benchCircuit
	circuit.n = n

	ccs, err := frontend.Compile(curveID, backend.PLONK, &circuit)
	if err != nil {
		panic(err)
	}

	witness := generateSolution(n, curveID)
	ww := witness_bn254.Witness{}
	if err := ww.FromFullAssignment(&witness); err != nil {
		panic(err)
	}

	cccs := ccs.(*cs_bn254.SparseR1CS)

	// measure proving time
	start := time.Now()
	p := profile.Start(profile.CPUProfile, profile.ProfilePath("."), profile.NoShutdownHook)
	_, err = cccs.Solve(ww)
	// _, err = plonk.Prove(ccs, pk, &witness)
	p.Stop()
	if err != nil {
		panic(err)
	}

	took := time.Since(start)
	fmt.Println("took", took.Milliseconds())
	// // check memory usage, max ram requested from OS
	// var m runtime.MemStats
	// runtime.ReadMemStats(&m)

	// bData := benchData{
	// 	Curve:          curveID.String(),
	// 	NbCores:        runtime.NumCPU(),
	// 	NbCoefficients: ccs.GetNbCoefficients(),
	// 	NbConstraints:  ccs.GetNbConstraints(),
	// 	NbWires:        0, // TODO @gbotrel fixme
	// 	RunTime:        took.Milliseconds(),
	// 	MaxRAM:         (m.Sys / 1024 / 1024),
	// 	Throughput:     int(float64(ccs.GetNbConstraints()) / took.Seconds()),
	// }
	// bData.ThroughputPerCore = bData.Throughput / bData.NbCores

	// if err := w.Write(bData.values()); err != nil {
	// 	panic(err)
	// }
	// w.Flush()

}

// benchCircuit is a simple circuit that checks X*X*X*X*X... == Y
type benchCircuit struct {
	X frontend.Variable
	Y frontend.Variable `gnark:",public"`
	n int
}

func (circuit *benchCircuit) Define(curveID ecc.ID, cs *frontend.ConstraintSystem) error {
	for i := 0; i < circuit.n; i++ {
		circuit.X = cs.Mul(circuit.X, circuit.X)
	}
	cs.AssertIsEqual(circuit.X, circuit.Y)
	return nil
}

func generateCircuit(nbConstraints int, curveID ecc.ID) (plonk.ProvingKey, frontend.CompiledConstraintSystem, *kzg.SRS) {
	var circuit benchCircuit
	circuit.n = nbConstraints

	ccs, err := frontend.Compile(curveID, backend.PLONK, &circuit)
	if err != nil {
		panic(err)
	}
	sparseR1CS := ccs.(*cs.SparseR1CS)
	nbConstraints_ := len(sparseR1CS.Constraints)
	nbVariables := sparseR1CS.NbInternalVariables + sparseR1CS.NbPublicVariables + sparseR1CS.NbSecretVariables
	var s uint64
	if nbConstraints_ > nbVariables {
		s = uint64(nbConstraints)
	} else {
		s = uint64(nbVariables)
	}

	srs, err := kzg.NewSRS(ecc.NextPowerOfTwo(s)+3, new(big.Int).SetInt64(42))
	if err != nil {
		panic(err)
	}
	// dummy setup will not compute a verifying key and just sets random value in the proving key
	pk, _, err := plonk.Setup(ccs, srs)
	if err != nil {
		panic(err)
	}

	return pk, ccs, srs
}

func generateSolution(nbConstraints int, curveID ecc.ID) (witness benchCircuit) {
	witness.n = nbConstraints
	witness.X.Assign(2)

	switch curveID {
	case ecc.BN254:
		// compute expected Y
		var expectedY bn254fr.Element
		expectedY.SetInterface(2)
		for i := 0; i < nbConstraints; i++ {
			expectedY.Mul(&expectedY, &expectedY)
		}

		witness.Y.Assign(expectedY)
	case ecc.BLS12_381:
		// compute expected Y
		var expectedY bls12381fr.Element
		expectedY.SetInterface(2)
		for i := 0; i < nbConstraints; i++ {
			expectedY.Mul(&expectedY, &expectedY)
		}

		witness.Y.Assign(expectedY)
	default:
		panic("not implemented")
	}

	return
}

type benchData struct {
	Curve             string
	NbConstraints     int
	NbWires           int
	NbCoefficients    int
	MaxRAM            uint64
	RunTime           int64
	NbCores           int
	Throughput        int
	ThroughputPerCore int
}

func (bData benchData) headers() []string {
	return []string{"curve", "nbConstraints", "nbWires", "nbCoefficients", "ram(mb)", "time(ms)", "nbCores", "throughput(constraints/s)", "througputPerCore(constraints/s)"}
}
func (bData benchData) values() []string {
	return []string{
		bData.Curve,
		strconv.Itoa(int(bData.NbConstraints)),
		strconv.Itoa(int(bData.NbWires)),
		strconv.Itoa(bData.NbCoefficients),
		strconv.Itoa(int(bData.MaxRAM)),
		strconv.Itoa(int(bData.RunTime)),
		strconv.Itoa(bData.NbCores),
		strconv.Itoa(bData.Throughput),
		strconv.Itoa(bData.ThroughputPerCore),
	}
}
