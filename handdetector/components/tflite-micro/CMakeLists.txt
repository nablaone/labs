# Upstream TensorFlow Lite Micro as an ESP-IDF component.
#
# The sources in tflm/ are generated, not committed: `make -C model tflm`
# checks out tflite-micro at a pinned commit (model/Makefile TFLM_COMMIT,
# also written to tflm/COMMIT) and runs its own create_tflm_tree.py --
# reference kernels only, no OPTIMIZED_KERNEL_DIR, so this is plain
# portable C++ with no vendor (CMSIS-NN / ESP-NN) code.

set(TFLM ${CMAKE_CURRENT_LIST_DIR}/tflm)
if(NOT EXISTS ${TFLM}/tensorflow/lite/micro/micro_interpreter.h)
    message(FATAL_ERROR "TFLM sources missing: run `make -C model tflm` first")
endif()

file(GLOB_RECURSE TFLM_SRCS
    ${TFLM}/tensorflow/*.cc ${TFLM}/tensorflow/*.c
    ${TFLM}/signal/*.cc ${TFLM}/signal/*.c
    ${TFLM}/third_party/*.cc ${TFLM}/third_party/*.c)

idf_component_register(
    SRCS ${TFLM_SRCS}
    INCLUDE_DIRS
        tflm
        tflm/third_party/flatbuffers/include
        tflm/third_party/kissfft)

target_compile_definitions(${COMPONENT_LIB} PUBLIC
    TF_LITE_STATIC_MEMORY          # no dynamic allocation inside TFLM
    TF_LITE_DISABLE_X86_NEON
    TF_LITE_USE_GLOBAL_CMATH_FUNCTIONS)

# Kernels are the hot path: -O2 regardless of the project's (debug)
# optimisation level. Upstream code isn't warning-clean under IDF's
# -Werror=all, so don't fail the build on its warnings.
target_compile_options(${COMPONENT_LIB} PRIVATE
    -O2 -Wno-error -Wno-unused-variable -Wno-unused-function
    -Wno-missing-field-initializers -Wno-maybe-uninitialized
    -Wno-sign-compare -Wno-nonnull)
