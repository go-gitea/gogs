// Copyright (c) 2015 Klaus Post, released under MIT License. See LICENSE file.

// Package cpuid provides information about the CPU running the current program.
//
// CPU features are detected on startup, and kept for fast access through the life of the application.
// Currently x86 / x64 (AMD64) as well as arm64 is supported.
//
// You can access the CPU information by accessing the shared CPU variable of the cpuid library.
//
// Package home: https://github.com/klauspost/cpuid
package cpuid

import (
	"flag"
	"fmt"
	"math"
	"os"
	"runtime"
	"strings"
)

// AMD refererence: https://www.amd.com/system/files/TechDocs/25481.pdf
// and Processor Programming Reference (PPR)

// Vendor is a representation of a CPU vendor.
type Vendor int

const (
	VendorUnknown Vendor = iota
	Intel
	AMD
	VIA
	Transmeta
	NSC
	KVM  // Kernel-based Virtual Machine
	MSVM // Microsoft Hyper-V or Windows Virtual PC
	VMware
	XenHVM
	Bhyve
	Hygon
	SiS
	RDC

	Ampere
	ARM
	Broadcom
	Cavium
	DEC
	Fujitsu
	Infineon
	Motorola
	NVIDIA
	AMCC
	Qualcomm
	Marvell

	lastVendor
)

//go:generate stringer -type=FeatureID,Vendor

// FeatureID is the ID of a specific cpu feature.
type FeatureID int

const (
	// Keep index -1 as unknown
	UNKNOWN = -1

	// Add features
	ADX                FeatureID = iota // Intel ADX (Multi-Precision Add-Carry Instruction Extensions)
	AESNI                               // Advanced Encryption Standard New Instructions
	AMD3DNOW                            // AMD 3DNOW
	AMD3DNOWEXT                         // AMD 3DNowExt
	AMXBF16                             // Tile computational operations on BFLOAT16 numbers
	AMXINT8                             // Tile computational operations on 8-bit integers
	AMXTILE                             // Tile architecture
	AVX                                 // AVX functions
	AVX2                                // AVX2 functions
	AVX512BF16                          // AVX-512 BFLOAT16 Instructions
	AVX512BITALG                        // AVX-512 Bit Algorithms
	AVX512BW                            // AVX-512 Byte and Word Instructions
	AVX512CD                            // AVX-512 Conflict Detection Instructions
	AVX512DQ                            // AVX-512 Doubleword and Quadword Instructions
	AVX512ER                            // AVX-512 Exponential and Reciprocal Instructions
	AVX512F                             // AVX-512 Foundation
	AVX512IFMA                          // AVX-512 Integer Fused Multiply-Add Instructions
	AVX512PF                            // AVX-512 Prefetch Instructions
	AVX512VBMI                          // AVX-512 Vector Bit Manipulation Instructions
	AVX512VBMI2                         // AVX-512 Vector Bit Manipulation Instructions, Version 2
	AVX512VL                            // AVX-512 Vector Length Extensions
	AVX512VNNI                          // AVX-512 Vector Neural Network Instructions
	AVX512VP2INTERSECT                  // AVX-512 Intersect for D/Q
	AVX512VPOPCNTDQ                     // AVX-512 Vector Population Count Doubleword and Quadword
	AVXSLOW                             // Indicates the CPU performs 2 128 bit operations instead of one.
	BMI1                                // Bit Manipulation Instruction Set 1
	BMI2                                // Bit Manipulation Instruction Set 2
	CLDEMOTE                            // Cache Line Demote
	CLMUL                               // Carry-less Multiplication
	CLZERO                              // CLZERO instruction supported
	CMOV                                // i686 CMOV
	CPBOOST                             // Core Performance Boost
	CX16                                // CMPXCHG16B Instruction
	ENQCMD                              // Enqueue Command
	ERMS                                // Enhanced REP MOVSB/STOSB
	F16C                                // Half-precision floating-point conversion
	FMA3                                // Intel FMA 3. Does not imply AVX.
	FMA4                                // Bulldozer FMA4 functions
	GFNI                                // Galois Field New Instructions
	HLE                                 // Hardware Lock Elision
	HTT                                 // Hyperthreading (enabled)
	HWA                                 // Hardware assert supported. Indicates support for MSRC001_10
	HYPERVISOR                          // This bit has been reserved by Intel & AMD for use by hypervisors
	IBPB                                // Indirect Branch Restricted Speculation (IBRS) and Indirect Branch Predictor Barrier (IBPB)
	IBS                                 // Instruction Based Sampling (AMD)
	IBSBRNTRGT                          // Instruction Based Sampling Feature (AMD)
	IBSFETCHSAM                         // Instruction Based Sampling Feature (AMD)
	IBSFFV                              // Instruction Based Sampling Feature (AMD)
	IBSOPCNT                            // Instruction Based Sampling Feature (AMD)
	IBSOPCNTEXT                         // Instruction Based Sampling Feature (AMD)
	IBSOPSAM                            // Instruction Based Sampling Feature (AMD)
	IBSRDWROPCNT                        // Instruction Based Sampling Feature (AMD)
	IBSRIPINVALIDCHK                    // Instruction Based Sampling Feature (AMD)
	INT_WBINVD                          // WBINVD/WBNOINVD are interruptible.
	INVLPGB                             // NVLPGB and TLBSYNC instruction supported
	LZCNT                               // LZCNT instruction
	MCAOVERFLOW                         // MCA overflow recovery support.
	MCOMMIT                             // MCOMMIT instruction supported
	MMX                                 // standard MMX
	MMXEXT                              // SSE integer functions or AMD MMX ext
	MOVDIR64B                           // Move 64 Bytes as Direct Store
	MOVDIRI                             // Move Doubleword as Direct Store
	MPX                                 // Intel MPX (Memory Protection Extensions)
	MSRIRC                              // Instruction Retired Counter MSR available
	NX                                  // NX (No-Execute) bit
	POPCNT                              // POPCNT instruction
	RDPRU                               // RDPRU instruction supported
	RDRAND                              // RDRAND instruction is available
	RDSEED                              // RDSEED instruction is available
	RDTSCP                              // RDTSCP Instruction
	RTM                                 // Restricted Transactional Memory
	RTM_ALWAYS_ABORT                    // Indicates that the loaded microcode is forcing RTM abort.
	SERIALIZE                           // Serialize Instruction Execution
	SGX                                 // Software Guard Extensions
	SGXLC                               // Software Guard Extensions Launch Control
	SHA                                 // Intel SHA Extensions
	SSE                                 // SSE functions
	SSE2                                // P4 SSE functions
	SSE3                                // Prescott SSE3 functions
	SSE4                                // Penryn SSE4.1 functions
	SSE42                               // Nehalem SSE4.2 functions
	SSE4A                               // AMD Barcelona microarchitecture SSE4a instructions
	SSSE3                               // Conroe SSSE3 functions
	STIBP                               // Single Thread Indirect Branch Predictors
	SUCCOR                              // Software uncorrectable error containment and recovery capability.
	TBM                                 // AMD Trailing Bit Manipulation
	TSXLDTRK                            // Intel TSX Suspend Load Address Tracking
	VAES                                // Vector AES
	VMX                                 // Virtual Machine Extensions
	VPCLMULQDQ                          // Carry-Less Multiplication Quadword
	WAITPKG                             // TPAUSE, UMONITOR, UMWAIT
	WBNOINVD                            // Write Back and Do Not Invalidate Cache
	XOP                                 // Bulldozer XOP functions

	// ARM features:
	AESARM   // AES instructions
	ARMCPUID // Some CPU ID registers readable at user-level
	ASIMD    // Advanced SIMD
	ASIMDDP  // SIMD Dot Product
	ASIMDHP  // Advanced SIMD half-precision floating point
	ASIMDRDM // Rounding Double Multiply Accumulate/Subtract (SQRDMLAH/SQRDMLSH)
	ATOMICS  // Large System Extensions (LSE)
	CRC32    // CRC32/CRC32C instructions
	DCPOP    // Data cache clean to Point of Persistence (DC CVAP)
	EVTSTRM  // Generic timer
	FCMA     // Floatin point complex number addition and multiplication
	FP       // Single-precision and double-precision floating point
	FPHP     // Half-precision floating point
	GPA      // Generic Pointer Authentication
	JSCVT    // Javascript-style double->int convert (FJCVTZS)
	LRCPC    // Weaker release consistency (LDAPR, etc)
	PMULL    // Polynomial Multiply instructions (PMULL/PMULL2)
	SHA1     // SHA-1 instructions (SHA1C, etc)
	SHA2     // SHA-2 instructions (SHA256H, etc)
	SHA3     // SHA-3 instructions (EOR3, RAXI, XAR, BCAX)
	SHA512   // SHA512 instructions
	SM3      // SM3 instructions
	SM4      // SM4 instructions
	SVE      // Scalable Vector Extension

	// Keep it last. It automatically defines the size of []flagSet
	lastID

	firstID FeatureID = UNKNOWN + 1
)

// CPUInfo contains information about the detected system CPU.
type CPUInfo struct {
	BrandName      string  // Brand name reported by the CPU
	VendorID       Vendor  // Comparable CPU vendor ID
	VendorString   string  // Raw vendor string.
	featureSet     flagSet // Features of the CPU
	PhysicalCores  int     // Number of physical processor cores in your CPU. Will be 0 if undetectable.
	ThreadsPerCore int     // Number of threads per physical core. Will be 1 if undetectable.
	LogicalCores   int     // Number of physical cores times threads that can run on each core through the use of hyperthreading. Will be 0 if undetectable.
	Family         int     // CPU family number
	Model          int     // CPU model number
	CacheLine      int     // Cache line size in bytes. Will be 0 if undetectable.
	Hz             int64   // Clock speed, if known, 0 otherwise. Will attempt to contain base clock speed.
	BoostFreq      int64   // Max clock speed, if known, 0 otherwise
	Cache          struct {
		L1I int // L1 Instruction Cache (per core or shared). Will be -1 if undetected
		L1D int // L1 Data Cache (per core or shared). Will be -1 if undetected
		L2  int // L2 Cache (per core or shared). Will be -1 if undetected
		L3  int // L3 Cache (per core, per ccx or shared). Will be -1 if undetected
	}
	SGX       SGXSupport
	maxFunc   uint32
	maxExFunc uint32
}

var cpuid func(op uint32) (eax, ebx, ecx, edx uint32)
var cpuidex func(op, op2 uint32) (eax, ebx, ecx, edx uint32)
var xgetbv func(index uint32) (eax, edx uint32)
var rdtscpAsm func() (eax, ebx, ecx, edx uint32)
var darwinHasAVX512 = func() bool { return false }

// CPU contains information about the CPU as detected on startup,
// or when Detect last was called.
//
// Use this as the primary entry point to you data.
var CPU CPUInfo

func init() {
	initCPU()
	Detect()
}

// Detect will re-detect current CPU info.
// This will replace the content of the exported CPU variable.
//
// Unless you expect the CPU to change while you are running your program
// you should not need to call this function.
// If you call this, you must ensure that no other goroutine is accessing the
// exported CPU variable.
func Detect() {
	// Set defaults
	CPU.ThreadsPerCore = 1
	CPU.Cache.L1I = -1
	CPU.Cache.L1D = -1
	CPU.Cache.L2 = -1
	CPU.Cache.L3 = -1
	safe := true
	if detectArmFlag != nil {
		safe = !*detectArmFlag
	}
	addInfo(&CPU, safe)
	if displayFeats != nil && *displayFeats {
		fmt.Println("cpu features:", strings.Join(CPU.FeatureSet(), ","))
		// Exit with non-zero so tests will print value.
		os.Exit(1)
	}
	if disableFlag != nil {
		s := strings.Split(*disableFlag, ",")
		for _, feat := range s {
			feat := ParseFeature(strings.TrimSpace(feat))
			if feat != UNKNOWN {
				CPU.featureSet.unset(feat)
			}
		}
	}
}

// DetectARM will detect ARM64 features.
// This is NOT done automatically since it can potentially crash
// if the OS does not handle the command.
// If in the future this can be done safely this function may not
// do anything.
func DetectARM() {
	addInfo(&CPU, false)
}

var detectArmFlag *bool
var displayFeats *bool
var disableFlag *string

// Flags will enable flags.
// This must be called *before* flag.Parse AND
// Detect must be called after the flags have been parsed.
// Note that this means that any detection used in init() functions
// will not contain these flags.
func Flags() {
	disableFlag = flag.String("cpu.disable", "", "disable cpu features; comma separated list")
	displayFeats = flag.Bool("cpu.features", false, "lists cpu features and exits")
	detectArmFlag = flag.Bool("cpu.arm", false, "allow ARM features to be detected; can potentially crash")
}

// Supports returns whether the CPU supports all of the requested features.
func (c CPUInfo) Supports(ids ...FeatureID) bool {
	for _, id := range ids {
		if !c.featureSet.inSet(id) {
			return false
		}
	}
	return true
}

// Has allows for checking a single feature.
// Should be inlined by the compiler.
func (c CPUInfo) Has(id FeatureID) bool {
	return c.featureSet.inSet(id)
}

// Disable will disable one or several features.
func (c *CPUInfo) Disable(ids ...FeatureID) bool {
	for _, id := range ids {
		c.featureSet.unset(id)
	}
	return true
}

// Enable will disable one or several features even if they were undetected.
// This is of course not recommended for obvious reasons.
func (c *CPUInfo) Enable(ids ...FeatureID) bool {
	for _, id := range ids {
		c.featureSet.set(id)
	}
	return true
}

// IsVendor returns true if vendor is recognized as Intel
func (c CPUInfo) IsVendor(v Vendor) bool {
	return c.VendorID == v
}

func (c CPUInfo) FeatureSet() []string {
	s := make([]string, 0)
	for _, f := range c.featureSet.Strings() {
		s = append(s, f)
	}
	return s
}

// RTCounter returns the 64-bit time-stamp counter
// Uses the RDTSCP instruction. The value 0 is returned
// if the CPU does not support the instruction.
func (c CPUInfo) RTCounter() uint64 {
	if !c.Supports(RDTSCP) {
		return 0
	}
	a, _, _, d := rdtscpAsm()
	return uint64(a) | (uint64(d) << 32)
}

// Ia32TscAux returns the IA32_TSC_AUX part of the RDTSCP.
// This variable is OS dependent, but on Linux contains information
// about the current cpu/core the code is running on.
// If the RDTSCP instruction isn't supported on the CPU, the value 0 is returned.
func (c CPUInfo) Ia32TscAux() uint32 {
	if !c.Supports(RDTSCP) {
		return 0
	}
	_, _, ecx, _ := rdtscpAsm()
	return ecx
}

// LogicalCPU will return the Logical CPU the code is currently executing on.
// This is likely to change when the OS re-schedules the running thread
// to another CPU.
// If the current core cannot be detected, -1 will be returned.
func (c CPUInfo) LogicalCPU() int {
	if c.maxFunc < 1 {
		return -1
	}
	_, ebx, _, _ := cpuid(1)
	return int(ebx >> 24)
}

// frequencies tries to compute the clock speed of the CPU. If leaf 15 is
// supported, use it, otherwise parse the brand string. Yes, really.
func (c *CPUInfo) frequencies() {
	c.Hz, c.BoostFreq = 0, 0
	mfi := maxFunctionID()
	if mfi >= 0x15 {
		eax, ebx, ecx, _ := cpuid(0x15)
		if eax != 0 && ebx != 0 && ecx != 0 {
			c.Hz = (int64(ecx) * int64(ebx)) / int64(eax)
		}
	}
	if mfi >= 0x16 {
		a, b, _, _ := cpuid(0x16)
		// Base...
		if a&0xffff > 0 {
			c.Hz = int64(a&0xffff) * 1_000_000
		}
		// Boost...
		if b&0xffff > 0 {
			c.BoostFreq = int64(b&0xffff) * 1_000_000
		}
	}
	if c.Hz > 0 {
		return
	}

	// computeHz determines the official rated speed of a CPU from its brand
	// string. This insanity is *actually the official documented way to do
	// this according to Intel*, prior to leaf 0x15 existing. The official
	// documentation only shows this working for exactly `x.xx` or `xxxx`
	// cases, e.g., `2.50GHz` or `1300MHz`; this parser will accept other
	// sizes.
	model := c.BrandName
	hz := strings.LastIndex(model, "Hz")
	if hz < 3 {
		return
	}
	var multiplier int64
	switch model[hz-1] {
	case 'M':
		multiplier = 1000 * 1000
	case 'G':
		multiplier = 1000 * 1000 * 1000
	case 'T':
		multiplier = 1000 * 1000 * 1000 * 1000
	}
	if multiplier == 0 {
		return
	}
	freq := int64(0)
	divisor := int64(0)
	decimalShift := int64(1)
	var i int
	for i = hz - 2; i >= 0 && model[i] != ' '; i-- {
		if model[i] >= '0' && model[i] <= '9' {
			freq += int64(model[i]-'0') * decimalShift
			decimalShift *= 10
		} else if model[i] == '.' {
			if divisor != 0 {
				return
			}
			divisor = decimalShift
		} else {
			return
		}
	}
	// we didn't find a space
	if i < 0 {
		return
	}
	if divisor != 0 {
		c.Hz = (freq * multiplier) / divisor
		return
	}
	c.Hz = freq * multiplier
}

// VM Will return true if the cpu id indicates we are in
// a virtual machine.
func (c CPUInfo) VM() bool {
	return CPU.featureSet.inSet(HYPERVISOR)
}

// flags contains detected cpu features and characteristics
type flags uint64

// log2(bits_in_uint64)
const flagBitsLog2 = 6
const flagBits = 1 << flagBitsLog2
const flagMask = flagBits - 1

// flagSet contains detected cpu features and characteristics in an array of flags
type flagSet [(lastID + flagMask) / flagBits]flags

func (s flagSet) inSet(feat FeatureID) bool {
	return s[feat>>flagBitsLog2]&(1<<(feat&flagMask)) != 0
}

func (s *flagSet) set(feat FeatureID) {
	s[feat>>flagBitsLog2] |= 1 << (feat & flagMask)
}

// setIf will set a feature if boolean is true.
func (s *flagSet) setIf(cond bool, features ...FeatureID) {
	if cond {
		for _, offset := range features {
			s[offset>>flagBitsLog2] |= 1 << (offset & flagMask)
		}
	}
}

func (s *flagSet) unset(offset FeatureID) {
	bit := flags(1 << (offset & flagMask))
	s[offset>>flagBitsLog2] = s[offset>>flagBitsLog2] & ^bit
}

// or with another flagset.
func (s *flagSet) or(other flagSet) {
	for i, v := range other[:] {
		s[i] |= v
	}
}

// ParseFeature will parse the string and return the ID of the matching feature.
// Will return UNKNOWN if not found.
func ParseFeature(s string) FeatureID {
	s = strings.ToUpper(s)
	for i := firstID; i < lastID; i++ {
		if i.String() == s {
			return i
		}
	}
	return UNKNOWN
}

// Strings returns an array of the detected features for FlagsSet.
func (s flagSet) Strings() []string {
	if len(s) == 0 {
		return []string{""}
	}
	r := make([]string, 0)
	for i := firstID; i < lastID; i++ {
		if s.inSet(i) {
			r = append(r, i.String())
		}
	}
	return r
}

func maxExtendedFunction() uint32 {
	eax, _, _, _ := cpuid(0x80000000)
	return eax
}

func maxFunctionID() uint32 {
	a, _, _, _ := cpuid(0)
	return a
}

func brandName() string {
	if maxExtendedFunction() >= 0x80000004 {
		v := make([]uint32, 0, 48)
		for i := uint32(0); i < 3; i++ {
			a, b, c, d := cpuid(0x80000002 + i)
			v = append(v, a, b, c, d)
		}
		return strings.Trim(string(valAsString(v...)), " ")
	}
	return "unknown"
}

func threadsPerCore() int {
	mfi := maxFunctionID()
	vend, _ := vendorID()

	if mfi < 0x4 || (vend != Intel && vend != AMD) {
		return 1
	}

	if mfi < 0xb {
		if vend != Intel {
			return 1
		}
		_, b, _, d := cpuid(1)
		if (d & (1 << 28)) != 0 {
			// v will contain logical core count
			v := (b >> 16) & 255
			if v > 1 {
				a4, _, _, _ := cpuid(4)
				// physical cores
				v2 := (a4 >> 26) + 1
				if v2 > 0 {
					return int(v) / int(v2)
				}
			}
		}
		return 1
	}
	_, b, _, _ := cpuidex(0xb, 0)
	if b&0xffff == 0 {
		if vend == AMD {
			// Workaround for AMD returning 0, assume 2 if >= Zen 2
			// It will be more correct than not.
			fam, _ := familyModel()
			_, _, _, d := cpuid(1)
			if (d&(1<<28)) != 0 && fam >= 23 {
				return 2
			}
		}
		return 1
	}
	return int(b & 0xffff)
}

func logicalCores() int {
	mfi := maxFunctionID()
	v, _ := vendorID()
	switch v {
	case Intel:
		// Use this on old Intel processors
		if mfi < 0xb {
			if mfi < 1 {
				return 0
			}
			// CPUID.1:EBX[23:16] represents the maximum number of addressable IDs (initial APIC ID)
			// that can be assigned to logical processors in a physical package.
			// The value may not be the same as the number of logical processors that are present in the hardware of a physical package.
			_, ebx, _, _ := cpuid(1)
			logical := (ebx >> 16) & 0xff
			return int(logical)
		}
		_, b, _, _ := cpuidex(0xb, 1)
		return int(b & 0xffff)
	case AMD, Hygon:
		_, b, _, _ := cpuid(1)
		return int((b >> 16) & 0xff)
	default:
		return 0
	}
}

func familyModel() (int, int) {
	if maxFunctionID() < 0x1 {
		return 0, 0
	}
	eax, _, _, _ := cpuid(1)
	family := ((eax >> 8) & 0xf) + ((eax >> 20) & 0xff)
	model := ((eax >> 4) & 0xf) + ((eax >> 12) & 0xf0)
	return int(family), int(model)
}

func physicalCores() int {
	v, _ := vendorID()
	switch v {
	case Intel:
		return logicalCores() / threadsPerCore()
	case AMD, Hygon:
		lc := logicalCores()
		tpc := threadsPerCore()
		if lc > 0 && tpc > 0 {
			return lc / tpc
		}

		// The following is inaccurate on AMD EPYC 7742 64-Core Processor
		if maxExtendedFunction() >= 0x80000008 {
			_, _, c, _ := cpuid(0x80000008)
			if c&0xff > 0 {
				return int(c&0xff) + 1
			}
		}
	}
	return 0
}

// Except from http://en.wikipedia.org/wiki/CPUID#EAX.3D0:_Get_vendor_ID
var vendorMapping = map[string]Vendor{
	"AMDisbetter!": AMD,
	"AuthenticAMD": AMD,
	"CentaurHauls": VIA,
	"GenuineIntel": Intel,
	"TransmetaCPU": Transmeta,
	"GenuineTMx86": Transmeta,
	"Geode by NSC": NSC,
	"VIA VIA VIA ": VIA,
	"KVMKVMKVMKVM": KVM,
	"Microsoft Hv": MSVM,
	"VMwareVMware": VMware,
	"XenVMMXenVMM": XenHVM,
	"bhyve bhyve ": Bhyve,
	"HygonGenuine": Hygon,
	"Vortex86 SoC": SiS,
	"SiS SiS SiS ": SiS,
	"RiseRiseRise": SiS,
	"Genuine  RDC": RDC,
}

func vendorID() (Vendor, string) {
	_, b, c, d := cpuid(0)
	v := string(valAsString(b, d, c))
	vend, ok := vendorMapping[v]
	if !ok {
		return VendorUnknown, v
	}
	return vend, v
}

func cacheLine() int {
	if maxFunctionID() < 0x1 {
		return 0
	}

	_, ebx, _, _ := cpuid(1)
	cache := (ebx & 0xff00) >> 5 // cflush size
	if cache == 0 && maxExtendedFunction() >= 0x80000006 {
		_, _, ecx, _ := cpuid(0x80000006)
		cache = ecx & 0xff // cacheline size
	}
	// TODO: Read from Cache and TLB Information
	return int(cache)
}

func (c *CPUInfo) cacheSize() {
	c.Cache.L1D = -1
	c.Cache.L1I = -1
	c.Cache.L2 = -1
	c.Cache.L3 = -1
	vendor, _ := vendorID()
	switch vendor {
	case Intel:
		if maxFunctionID() < 4 {
			return
		}
		for i := uint32(0); ; i++ {
			eax, ebx, ecx, _ := cpuidex(4, i)
			cacheType := eax & 15
			if cacheType == 0 {
				break
			}
			cacheLevel := (eax >> 5) & 7
			coherency := int(ebx&0xfff) + 1
			partitions := int((ebx>>12)&0x3ff) + 1
			associativity := int((ebx>>22)&0x3ff) + 1
			sets := int(ecx) + 1
			size := associativity * partitions * coherency * sets
			switch cacheLevel {
			case 1:
				if cacheType == 1 {
					// 1 = Data Cache
					c.Cache.L1D = size
				} else if cacheType == 2 {
					// 2 = Instruction Cache
					c.Cache.L1I = size
				} else {
					if c.Cache.L1D < 0 {
						c.Cache.L1I = size
					}
					if c.Cache.L1I < 0 {
						c.Cache.L1I = size
					}
				}
			case 2:
				c.Cache.L2 = size
			case 3:
				c.Cache.L3 = size
			}
		}
	case AMD, Hygon:
		// Untested.
		if maxExtendedFunction() < 0x80000005 {
			return
		}
		_, _, ecx, edx := cpuid(0x80000005)
		c.Cache.L1D = int(((ecx >> 24) & 0xFF) * 1024)
		c.Cache.L1I = int(((edx >> 24) & 0xFF) * 1024)

		if maxExtendedFunction() < 0x80000006 {
			return
		}
		_, _, ecx, _ = cpuid(0x80000006)
		c.Cache.L2 = int(((ecx >> 16) & 0xFFFF) * 1024)

		// CPUID Fn8000_001D_EAX_x[N:0] Cache Properties
		if maxExtendedFunction() < 0x8000001D {
			return
		}
		for i := uint32(0); i < math.MaxUint32; i++ {
			eax, ebx, ecx, _ := cpuidex(0x8000001D, i)

			level := (eax >> 5) & 7
			cacheNumSets := ecx + 1
			cacheLineSize := 1 + (ebx & 2047)
			cachePhysPartitions := 1 + ((ebx >> 12) & 511)
			cacheNumWays := 1 + ((ebx >> 22) & 511)

			typ := eax & 15
			size := int(cacheNumSets * cacheLineSize * cachePhysPartitions * cacheNumWays)
			if typ == 0 {
				return
			}

			switch level {
			case 1:
				switch typ {
				case 1:
					// Data cache
					c.Cache.L1D = size
				case 2:
					// Inst cache
					c.Cache.L1I = size
				default:
					if c.Cache.L1D < 0 {
						c.Cache.L1I = size
					}
					if c.Cache.L1I < 0 {
						c.Cache.L1I = size
					}
				}
			case 2:
				c.Cache.L2 = size
			case 3:
				c.Cache.L3 = size
			}
		}
	}

	return
}

type SGXEPCSection struct {
	BaseAddress uint64
	EPCSize     uint64
}

type SGXSupport struct {
	Available           bool
	LaunchControl       bool
	SGX1Supported       bool
	SGX2Supported       bool
	MaxEnclaveSizeNot64 int64
	MaxEnclaveSize64    int64
	EPCSections         []SGXEPCSection
}

func hasSGX(available, lc bool) (rval SGXSupport) {
	rval.Available = available

	if !available {
		return
	}

	rval.LaunchControl = lc

	a, _, _, d := cpuidex(0x12, 0)
	rval.SGX1Supported = a&0x01 != 0
	rval.SGX2Supported = a&0x02 != 0
	rval.MaxEnclaveSizeNot64 = 1 << (d & 0xFF)     // pow 2
	rval.MaxEnclaveSize64 = 1 << ((d >> 8) & 0xFF) // pow 2
	rval.EPCSections = make([]SGXEPCSection, 0)

	for subleaf := uint32(2); subleaf < 2+8; subleaf++ {
		eax, ebx, ecx, edx := cpuidex(0x12, subleaf)
		leafType := eax & 0xf

		if leafType == 0 {
			// Invalid subleaf, stop iterating
			break
		} else if leafType == 1 {
			// EPC Section subleaf
			baseAddress := uint64(eax&0xfffff000) + (uint64(ebx&0x000fffff) << 32)
			size := uint64(ecx&0xfffff000) + (uint64(edx&0x000fffff) << 32)

			section := SGXEPCSection{BaseAddress: baseAddress, EPCSize: size}
			rval.EPCSections = append(rval.EPCSections, section)
		}
	}

	return
}

func support() flagSet {
	var fs flagSet
	mfi := maxFunctionID()
	vend, _ := vendorID()
	if mfi < 0x1 {
		return fs
	}
	family, model := familyModel()

	_, _, c, d := cpuid(1)
	fs.setIf((d&(1<<15)) != 0, CMOV)
	fs.setIf((d&(1<<23)) != 0, MMX)
	fs.setIf((d&(1<<25)) != 0, MMXEXT)
	fs.setIf((d&(1<<25)) != 0, SSE)
	fs.setIf((d&(1<<26)) != 0, SSE2)
	fs.setIf((c&1) != 0, SSE3)
	fs.setIf((c&(1<<5)) != 0, VMX)
	fs.setIf((c&0x00000200) != 0, SSSE3)
	fs.setIf((c&0x00080000) != 0, SSE4)
	fs.setIf((c&0x00100000) != 0, SSE42)
	fs.setIf((c&(1<<25)) != 0, AESNI)
	fs.setIf((c&(1<<1)) != 0, CLMUL)
	fs.setIf(c&(1<<23) != 0, POPCNT)
	fs.setIf(c&(1<<30) != 0, RDRAND)

	// This bit has been reserved by Intel & AMD for use by hypervisors,
	// and indicates the presence of a hypervisor.
	fs.setIf(c&(1<<31) != 0, HYPERVISOR)
	fs.setIf(c&(1<<29) != 0, F16C)
	fs.setIf(c&(1<<13) != 0, CX16)

	if vend == Intel && (d&(1<<28)) != 0 && mfi >= 4 {
		fs.setIf(threadsPerCore() > 1, HTT)
	}
	if vend == AMD && (d&(1<<28)) != 0 && mfi >= 4 {
		fs.setIf(threadsPerCore() > 1, HTT)
	}
	// Check XGETBV/XSAVE (26), OXSAVE (27) and AVX (28) bits
	const avxCheck = 1<<26 | 1<<27 | 1<<28
	if c&avxCheck == avxCheck {
		// Check for OS support
		eax, _ := xgetbv(0)
		if (eax & 0x6) == 0x6 {
			fs.set(AVX)
			switch vend {
			case Intel:
				// Older than Haswell.
				fs.setIf(family == 6 && model < 60, AVXSLOW)
			case AMD:
				// Older than Zen 2
				fs.setIf(family < 23 || (family == 23 && model < 49), AVXSLOW)
			}
		}
	}
	// FMA3 can be used with SSE registers, so no OS support is strictly needed.
	// fma3 and OSXSAVE needed.
	const fma3Check = 1<<12 | 1<<27
	fs.setIf(c&fma3Check == fma3Check, FMA3)

	// Check AVX2, AVX2 requires OS support, but BMI1/2 don't.
	if mfi >= 7 {
		_, ebx, ecx, edx := cpuidex(7, 0)
		eax1, _, _, _ := cpuidex(7, 1)
		if fs.inSet(AVX) && (ebx&0x00000020) != 0 {
			fs.set(AVX2)
		}
		// CPUID.(EAX=7, ECX=0).EBX
		if (ebx & 0x00000008) != 0 {
			fs.set(BMI1)
			fs.setIf((ebx&0x00000100) != 0, BMI2)
		}
		fs.setIf(ebx&(1<<2) != 0, SGX)
		fs.setIf(ebx&(1<<4) != 0, HLE)
		fs.setIf(ebx&(1<<9) != 0, ERMS)
		fs.setIf(ebx&(1<<11) != 0, RTM)
		fs.setIf(ebx&(1<<14) != 0, MPX)
		fs.setIf(ebx&(1<<18) != 0, RDSEED)
		fs.setIf(ebx&(1<<19) != 0, ADX)
		fs.setIf(ebx&(1<<29) != 0, SHA)
		// CPUID.(EAX=7, ECX=0).ECX
		fs.setIf(ecx&(1<<5) != 0, WAITPKG)
		fs.setIf(ecx&(1<<25) != 0, CLDEMOTE)
		fs.setIf(ecx&(1<<27) != 0, MOVDIRI)
		fs.setIf(ecx&(1<<28) != 0, MOVDIR64B)
		fs.setIf(ecx&(1<<29) != 0, ENQCMD)
		fs.setIf(ecx&(1<<30) != 0, SGXLC)
		// CPUID.(EAX=7, ECX=0).EDX
		fs.setIf(edx&(1<<11) != 0, RTM_ALWAYS_ABORT)
		fs.setIf(edx&(1<<14) != 0, SERIALIZE)
		fs.setIf(edx&(1<<16) != 0, TSXLDTRK)
		fs.setIf(edx&(1<<26) != 0, IBPB)
		fs.setIf(edx&(1<<27) != 0, STIBP)

		// Only detect AVX-512 features if XGETBV is supported
		if c&((1<<26)|(1<<27)) == (1<<26)|(1<<27) {
			// Check for OS support
			eax, _ := xgetbv(0)

			// Verify that XCR0[7:5] = ‘111b’ (OPMASK state, upper 256-bit of ZMM0-ZMM15 and
			// ZMM16-ZMM31 state are enabled by OS)
			/// and that XCR0[2:1] = ‘11b’ (XMM state and YMM state are enabled by OS).
			hasAVX512 := (eax>>5)&7 == 7 && (eax>>1)&3 == 3
			if runtime.GOOS == "darwin" {
				hasAVX512 = fs.inSet(AVX) && darwinHasAVX512()
			}
			if hasAVX512 {
				fs.setIf(ebx&(1<<16) != 0, AVX512F)
				fs.setIf(ebx&(1<<17) != 0, AVX512DQ)
				fs.setIf(ebx&(1<<21) != 0, AVX512IFMA)
				fs.setIf(ebx&(1<<26) != 0, AVX512PF)
				fs.setIf(ebx&(1<<27) != 0, AVX512ER)
				fs.setIf(ebx&(1<<28) != 0, AVX512CD)
				fs.setIf(ebx&(1<<30) != 0, AVX512BW)
				fs.setIf(ebx&(1<<31) != 0, AVX512VL)
				// ecx
				fs.setIf(ecx&(1<<1) != 0, AVX512VBMI)
				fs.setIf(ecx&(1<<6) != 0, AVX512VBMI2)
				fs.setIf(ecx&(1<<8) != 0, GFNI)
				fs.setIf(ecx&(1<<9) != 0, VAES)
				fs.setIf(ecx&(1<<10) != 0, VPCLMULQDQ)
				fs.setIf(ecx&(1<<11) != 0, AVX512VNNI)
				fs.setIf(ecx&(1<<12) != 0, AVX512BITALG)
				fs.setIf(ecx&(1<<14) != 0, AVX512VPOPCNTDQ)
				// edx
				fs.setIf(edx&(1<<8) != 0, AVX512VP2INTERSECT)
				fs.setIf(edx&(1<<22) != 0, AMXBF16)
				fs.setIf(edx&(1<<24) != 0, AMXTILE)
				fs.setIf(edx&(1<<25) != 0, AMXINT8)
				// eax1 = CPUID.(EAX=7, ECX=1).EAX
				fs.setIf(eax1&(1<<5) != 0, AVX512BF16)
			}
		}
	}

	if maxExtendedFunction() >= 0x80000001 {
		_, _, c, d := cpuid(0x80000001)
		if (c & (1 << 5)) != 0 {
			fs.set(LZCNT)
			fs.set(POPCNT)
		}
		fs.setIf((c&(1<<10)) != 0, IBS)
		fs.setIf((d&(1<<31)) != 0, AMD3DNOW)
		fs.setIf((d&(1<<30)) != 0, AMD3DNOWEXT)
		fs.setIf((d&(1<<23)) != 0, MMX)
		fs.setIf((d&(1<<22)) != 0, MMXEXT)
		fs.setIf((c&(1<<6)) != 0, SSE4A)
		fs.setIf(d&(1<<20) != 0, NX)
		fs.setIf(d&(1<<27) != 0, RDTSCP)

		/* XOP and FMA4 use the AVX instruction coding scheme, so they can't be
		 * used unless the OS has AVX support. */
		if fs.inSet(AVX) {
			fs.setIf((c&0x00000800) != 0, XOP)
			fs.setIf((c&0x00010000) != 0, FMA4)
		}

	}
	if maxExtendedFunction() >= 0x80000007 {
		_, b, _, d := cpuid(0x80000007)
		fs.setIf((b&(1<<0)) != 0, MCAOVERFLOW)
		fs.setIf((b&(1<<1)) != 0, SUCCOR)
		fs.setIf((b&(1<<2)) != 0, HWA)
		fs.setIf((d&(1<<9)) != 0, CPBOOST)
	}

	if maxExtendedFunction() >= 0x80000008 {
		_, b, _, _ := cpuid(0x80000008)
		fs.setIf((b&(1<<9)) != 0, WBNOINVD)
		fs.setIf((b&(1<<8)) != 0, MCOMMIT)
		fs.setIf((b&(1<<13)) != 0, INT_WBINVD)
		fs.setIf((b&(1<<4)) != 0, RDPRU)
		fs.setIf((b&(1<<3)) != 0, INVLPGB)
		fs.setIf((b&(1<<1)) != 0, MSRIRC)
		fs.setIf((b&(1<<0)) != 0, CLZERO)
	}

	if maxExtendedFunction() >= 0x8000001b && fs.inSet(IBS) {
		eax, _, _, _ := cpuid(0x8000001b)
		fs.setIf((eax>>0)&1 == 1, IBSFFV)
		fs.setIf((eax>>1)&1 == 1, IBSFETCHSAM)
		fs.setIf((eax>>2)&1 == 1, IBSOPSAM)
		fs.setIf((eax>>3)&1 == 1, IBSRDWROPCNT)
		fs.setIf((eax>>4)&1 == 1, IBSOPCNT)
		fs.setIf((eax>>5)&1 == 1, IBSBRNTRGT)
		fs.setIf((eax>>6)&1 == 1, IBSOPCNTEXT)
		fs.setIf((eax>>7)&1 == 1, IBSRIPINVALIDCHK)
	}

	return fs
}

func valAsString(values ...uint32) []byte {
	r := make([]byte, 4*len(values))
	for i, v := range values {
		dst := r[i*4:]
		dst[0] = byte(v & 0xff)
		dst[1] = byte((v >> 8) & 0xff)
		dst[2] = byte((v >> 16) & 0xff)
		dst[3] = byte((v >> 24) & 0xff)
		switch {
		case dst[0] == 0:
			return r[:i*4]
		case dst[1] == 0:
			return r[:i*4+1]
		case dst[2] == 0:
			return r[:i*4+2]
		case dst[3] == 0:
			return r[:i*4+3]
		}
	}
	return r
}
