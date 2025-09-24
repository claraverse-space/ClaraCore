# ClaraCore Platform Support

ClaraCore now provides comprehensive cross-platform support with optimal acceleration for Windows, macOS, and Linux.

## 🌍 Platform Matrix

### 🪟 Windows Support
- **✅ NVIDIA CUDA** - Best performance for RTX 40/30/20 series, GTX cards
- **✅ AMD ROCm** - AMD GPU acceleration for RX 7000/6000/5000 series  
- **✅ Vulkan** - Cross-platform GPU acceleration
- **✅ Intel GPU** - Integrated graphics acceleration
- **✅ CPU** - Multithreaded fallback
- **🎯 Priority**: CUDA > ROCm > Vulkan > CPU

### 🐧 Linux Support  
- **✅ NVIDIA CUDA** - Optimal for data centers & gaming rigs
- **✅ AMD ROCm** - Open-source AMD GPU acceleration
- **✅ Vulkan** - Modern GPU API support
- **✅ Intel GPU** - Integrated & discrete Intel graphics
- **✅ CPU** - Excellent Linux optimization
- **🎯 Priority**: CUDA > ROCm > Vulkan > CPU

### 🍎 macOS Support
- **✅ Apple MLX** - Apple Silicon unified memory (M1/M2/M3/M4)
- **✅ Metal** - Apple GPU acceleration framework
- **✅ Vulkan (MoltenVK)** - Cross-platform compatibility layer
- **✅ Intel GPU** - Intel Mac integrated graphics  
- **✅ CPU** - macOS-optimized processing
- **🎯 Priority**: Metal+MLX > Vulkan > CPU

## 🔧 Hardware Recommendations

### Best Performance
- **Windows/Linux**: NVIDIA RTX 4090 (24GB) / RTX 4080 (16GB)
- **macOS**: Apple M3 Max (128GB) / M2 Ultra (192GB)

### Great Performance  
- AMD RX 7900XTX (24GB) / RTX 3080 Ti (12GB)

### Good Performance
- RTX 3060 (12GB) / Intel Arc A770 (16GB)

### Budget Option
- CPU-only with 32GB+ RAM

## 📊 Model Size vs Hardware

| Model Size | VRAM Required | Apple Unified Memory |
|------------|---------------|---------------------|
| 70B+ models | 24GB+ VRAM | 64GB+ unified memory |
| 30B models | 16GB+ VRAM | 32GB+ unified memory |
| 13B models | 8GB+ VRAM | 16GB+ unified memory |
| 7B models | 6GB+ VRAM | 8GB+ unified memory |
| 3B models | 4GB+ VRAM | 4GB+ unified memory |

## 🚀 Automatic Detection Features

### Windows
- **CUDA Detection**: Automatic nvidia-smi detection and driver verification
- **ROCm Detection**: ROCm installation and AMD GPU detection
- **Vulkan Detection**: vulkan-1.dll and device verification
- **Intel GPU**: WMI-based detection with memory estimation

### Linux  
- **CUDA Detection**: nvidia-smi verification with device enumeration
- **ROCm Detection**: rocm-smi and AMD GPU driver verification
- **Vulkan Detection**: libvulkan.so detection with device verification
- **Intel GPU**: lspci-based detection with driver verification

### macOS
- **Metal Detection**: Framework verification with GPU compatibility
- **MLX Detection**: Apple Silicon unified memory estimation
- **MoltenVK**: Homebrew and system installation detection
- **Intel GPU**: Intel Mac integrated graphics detection

## 🎯 Binary Selection Logic

ClaraCore automatically downloads the optimal binary for your platform:

### Windows Binaries
1. `llama-b6527-bin-win-cuda-12.4-x64.zip` (CUDA + runtime)
2. `llama-b6527-bin-win-rocm-x64.zip` (AMD ROCm)
3. `llama-b6527-bin-win-vulkan-x64.zip` (Vulkan)  
4. `llama-b6527-bin-win-cpu-x64.zip` (CPU fallback)

### Linux Binaries
1. `llama-b6527-bin-ubuntu-x64.zip` (CUDA)
2. `llama-b6527-bin-ubuntu-rocm-x64.zip` (AMD ROCm)
3. `llama-b6527-bin-ubuntu-vulkan-x64.zip` (Vulkan)
4. `llama-b6527-bin-ubuntu-x64.zip` (CPU fallback)

### macOS Binaries
1. `llama-b6527-bin-macos-arm64.zip` (Apple Silicon + Metal)
2. `llama-b6527-bin-macos-x64.zip` (Intel Mac CPU)

## 💡 Installation Notes

### Windows
- Automatic driver detection and binary selection
- CUDA runtime automatically downloaded with CUDA binary
- ROCm drivers should be installed manually for AMD GPUs

### Linux
- Install CUDA/ROCm drivers manually for best performance  
- Package managers can install drivers: `sudo apt install nvidia-driver-535`
- ROCm: `sudo apt install rocm-dkms rocm-libs`

### macOS
- Metal/MLX work out-of-the-box on Apple Silicon
- MoltenVK can be installed via Homebrew: `brew install molten-vk`
- No additional setup required for Apple Silicon Macs

## 🧠 Smart Memory Management

### GPU Memory Detection
- **NVIDIA**: nvidia-smi for exact VRAM amounts
- **AMD**: rocm-smi for GPU memory detection  
- **Apple**: Unified memory estimation based on chip type
- **Intel**: Shared memory estimation based on system RAM

### Memory Allocation Strategy
- **Layer Distribution**: Smart GPU/CPU layer allocation
- **Context Optimization**: Dynamic context sizing based on available memory
- **KV Cache**: Intelligent quantization (f16/q8_0/q4_0) based on memory constraints
- **Hybrid Support**: CPU+GPU allocation for large models

## 🔍 Real-time Hardware Monitoring

Optional real-time monitoring provides:
- Current available VRAM (not total)
- Available system RAM after OS overhead
- Dynamic memory allocation adjustments
- Live hardware utilization feedback

Enable with `--enable-realtime` flag during auto-setup.

## 🎮 Gaming Performance Notes

### NVIDIA RTX Cards (Windows/Linux)
- RTX 4090: Handles 70B models at 24GB VRAM
- RTX 4080: Perfect for 30B models at 16GB VRAM  
- RTX 3080: Great for 13B models at 10-12GB VRAM

### AMD Cards (Windows/Linux)
- RX 7900XTX: Excellent 70B support with 24GB VRAM
- RX 6800XT: Good for 13B models with 16GB VRAM

### Apple Silicon (macOS)
- M3 Max: 128GB unified memory handles massive models
- M2 Ultra: 192GB unified memory for enterprise workloads
- M1 Pro/Max: 16-32GB unified memory for 7B-13B models

## 🚨 Troubleshooting

### Common Issues
1. **CUDA not detected**: Install latest NVIDIA drivers
2. **ROCm issues**: Verify AMD GPU driver installation
3. **Vulkan missing**: Install graphics drivers with Vulkan support
4. **Metal unavailable**: Update to latest macOS version

### Performance Optimization
- **Windows**: Use CUDA for NVIDIA, ROCm for AMD
- **Linux**: Install proprietary drivers for best performance
- **macOS**: Use Metal backend on Apple Silicon

### Memory Issues
- Enable real-time monitoring for accurate allocation
- Use CPU+GPU hybrid for models larger than VRAM
- Adjust context size based on available memory

---

**Built for**: Windows 10/11, Ubuntu 20.04+, macOS 12.0+
**Updated**: December 2024
**Version**: ClaraCore v1.0+