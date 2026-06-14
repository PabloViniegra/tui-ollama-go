// Package eval clasifica cada modelo según si cabe (y cómo de bien) en la
// memoria del equipo, usando una heurística RAM/VRAM-vs-tamaño.
package eval

import (
	"ollama-fit/internal/catalog"
	"ollama-fit/internal/hardware"
)

// Verdict es el veredicto para un modelo.
type Verdict int

const (
	Good  Verdict = iota // cabe con holgura -> rinde bien
	Tight                // cabe justo o con offload -> usable pero lento
	No                   // no cabe en memoria
)

// Result es la evaluación de un modelo concreto en un equipo concreto.
type Result struct {
	Model   catalog.Model
	Verdict Verdict
	Backend string  // GPU CUDA | GPU Metal | GPU+CPU | CPU | —
	NeedGB  float64 // memoria estimada necesaria
	Reason  string
}

// overhead añade un margen sobre el tamaño de los pesos para la KV-cache,
// el contexto y el runtime. Ajustable.
const overhead = 1.2

// AppleGPUFraction: fracción de la RAM unificada que Metal puede dedicar a GPU.
const AppleGPUFraction = 0.70

// thresholds de memoria para la heurística.
const (
	marginGPUGood    = 0.85 // por debajo de este % de VRAM -> Good
	marginRAMTight   = 0.90 // por debajo de este % de RAM -> Tight
	cpuGoodThreshold = 6.0  // GB por debajo de los cuales CPU es Good
)

// String devuelve el nombre en inglés del veredicto.
func (v Verdict) String() string {
	switch v {
	case Good:
		return "Good"
	case Tight:
		return "Tight"
	case No:
		return "No"
	default:
		return "Unknown"
	}
}

// Evaluate aplica la heurística.
func Evaluate(h hardware.Info, m catalog.Model) Result {
	need := m.SizeGB * overhead
	r := Result{Model: m, NeedGB: need}

	// Memoria acelerada disponible y su etiqueta.
	var gpuMem float64
	var gpuBackend string
	switch {
	case h.GPU.VRAMGB > 0:
		gpuMem = h.GPU.VRAMGB
		gpuBackend = "GPU " + gpuLabel(h.GPU.Kind)
	case h.AppleUnified:
		gpuMem = AppleGPUFraction * h.RAMGB
		gpuBackend = "GPU Metal"
	}

	// 1) Cabe con holgura en la GPU.
	if gpuMem > 0 && need <= marginGPUGood*gpuMem {
		r.Verdict, r.Backend, r.Reason = Good, gpuBackend, "Acelerado en GPU con margen"
		return r
	}
	// 2) Cabe justo en la GPU.
	if gpuMem > 0 && need <= gpuMem {
		r.Verdict, r.Backend, r.Reason = Tight, gpuBackend, "Cabe justo en GPU/VRAM"
		return r
	}
	// 3) No entra entero en GPU: miramos la RAM del sistema.
	if need <= AppleGPUFraction*h.RAMGB {
		if gpuMem > 0 {
			r.Verdict, r.Backend, r.Reason = Tight, "GPU+CPU", "Se reparte GPU/CPU, más lento"
		} else if need <= cpuGoodThreshold {
			r.Verdict, r.Backend, r.Reason = Good, "CPU", "Cabe en RAM, fluido en CPU"
		} else {
			r.Verdict, r.Backend, r.Reason = Tight, "CPU", "Cabe en RAM pero lento en CPU"
		}
		return r
	}
	// 4) Apurando la RAM.
	if need <= marginRAMTight*h.RAMGB {
		r.Verdict, r.Backend, r.Reason = Tight, "CPU", "Apura la RAM, muy lento"
		return r
	}
	// 5) No cabe.
	r.Verdict, r.Backend, r.Reason = No, "—", "No cabe en memoria"
	return r
}

func gpuLabel(kind hardware.GPUKind) string {
	switch kind {
	case hardware.GPUKindNVIDIA:
		return "CUDA"
	case hardware.GPUKindAMD:
		return "ROCm"
	case hardware.GPUKindApple:
		return "Metal"
	case hardware.GPUKindIntel:
		return "iGPU"
	}
	return ""
}
