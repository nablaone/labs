#include <stdio.h>
#include <string.h>

#include "esp_heap_caps.h"

#include "tensorflow/lite/micro/schema/schema_generated.h"

#include "pie_fusion.h"
#include "pie_kernels.h"

namespace {

constexpr int kMaxFused = 64;
PieFused *fused[kMaxFused];
int n_fused;
uint8_t *consumers;  /* per tensor, saturating */
int n_tensors;
bool enabled = true;

}  // namespace

void pie_fusion_set_model(const void *m)
{
	const tflite::Model *model = static_cast<const tflite::Model *>(m);
	const tflite::SubGraph *g = model->subgraphs()->Get(0);
	n_tensors = g->tensors()->size();
	heap_caps_free(consumers);
	consumers = (uint8_t *)heap_caps_calloc(n_tensors, 1, MALLOC_CAP_SPIRAM);
	if (!consumers) return;
	auto bump = [](int t) { if (t >= 0 && t < n_tensors && consumers[t] < 255) consumers[t]++; };
	for (const tflite::Operator *op : *g->operators())
		for (int t : *op->inputs()) bump(t);
	for (int t : *g->outputs()) bump(t);  /* a graph output counts as a reader */
}

int pie_fusion_consumers(int t)
{
	return consumers && t >= 0 && t < n_tensors ? consumers[t] : 255;
}

PieFused *pie_fusion_get(int conv_out, bool create)
{
	for (int i = 0; i < n_fused; i++)
		if (fused[i]->conv_out == conv_out) return fused[i];
	if (!create || n_fused == kMaxFused) return nullptr;
	PieFused *f = (PieFused *)heap_caps_calloc(1, sizeof(PieFused), MALLOC_CAP_SPIRAM);
	if (!f) return nullptr;
	f->conv_out = conv_out;
	fused[n_fused++] = f;
	return f;
}

PieFused *pie_fusion_by_add_out(int t)
{
	for (int i = 0; i < n_fused; i++)
		if (fused[i]->add && fused[i]->add_out == t) return fused[i];
	return nullptr;
}

void pie_fusion_set_enabled(bool on) { enabled = on; }
bool pie_fusion_enabled(void) { return enabled && pie_enabled() && !pie_verify_enabled(); }

void pie_fusion_print(void)
{
	int add_prelu = 0, add_only = 0, prelu_only = 0;
	for (int i = 0; i < n_fused; i++) {
		const PieFused *f = fused[i];
		add_prelu += f->add && f->prelu;
		add_only += f->add && !f->prelu;
		prelu_only += !f->add && f->prelu;
	}
	printf("fusion %s: CONV+ADD+PRELU %d, CONV+ADD %d, CONV+PRELU %d\n",
	       pie_fusion_enabled() ? "on" : "off", add_prelu, add_only, prelu_only);
}
