标题：InternVL: Scaling up Vision Foundation Models and Aligning for Generic Visual-Linguistic Tasks
机构：OpenGVLab, Shanghai AI Laboratory（上海人工智能实验室）
arXiv：2312.14238
来源：https://arxiv.org/abs/2312.14238
说明：以下摘自 arXiv 公开 HTML 正文（自摘要起，去掉页面装饰与文末参考文献），供工作区问答测试。

Abstract † † † \dagger This work is done when they are interns at Shanghai AI Laboratory;
🖂 corresponding author (daijifeng@tsinghua.edu.cn) 
The exponential growth of large language models (LLMs) has opened up numerous possibilities for multi-modal AGI systems.
However, the progress in vision and vision-language foundation models, which are also critical elements of multi-modal AGI, has not kept pace with LLMs.
In this work, we design a large-scale vision-language foundation model (InternVL), which scales up the vision foundation model to 6 billion parameters and progressively aligns it with the LLM, using web-scale image-text data from various sources.
This model can be broadly applied to and achieve state-of-the-art performance on 32 generic visual-linguistic benchmarks including visual perception tasks such as image-level or pixel-level recognition, vision-language tasks such as zero-shot image/video classification, zero-shot image/video-text retrieval, and link with LLMs to create multi-modal dialogue systems. It has powerful visual capabilities and can be a good alternative to the ViT-22B.
We hope that our research could contribute to the development of multi-modal large models. 

Figure 1 : Comparisons of different vision and vision-language foundation models. (a) indicates the traditional vision foundation model, e.g . ResNet [ 57 ] pre-trained on classification tasks. (b) represents the vision-language foundation models, e.g . CLIP [ 117 ] pre-trained on image-text pairs. (c) is our InternVL, which presents a workable way to align the large-scale vision foundation model ( i.e . , InternViT-6B) with the large language model and is versatile for both contrastive and generative tasks. 
1 Introduction 

Large language models (LLMs) largely promote the development of artificial general intelligence (AGI) systems with their impressive capabilities in open-world language tasks, and their model scale and performance are still increasing at a fast pace.
Vision large language models (VLLMs) [ 147 , 5 , 92 , 187 , 21 , 115 , 34 , 3 , 23 ] , which leverage LLMs, have also achieved significant breakthroughs, enabling sophisticated vision-language dialogues and interactions.
However, the progress of vision and vision-language foundation models, which are also crucial for VLLMs, has lagged behind the rapid growth of LLMs. 

Figure 2 : Comparison results on various generic visual-linguistic tasks , including image classification, video classification, image-text retrieval, image captioning, and multi-modal dialogue. The proposed InternVL achieves the best performance on all these tasks. Note that only the models trained on public data are included.
“IN” is an abbreviation for ImageNet [ 38 ] . 

To bridge vision models with LLMs, existing VLLMs [ 81 , 187 , 5 , 177 , 131 ] commonly employ lightweight “glue” layers, such as QFormer [ 81 ] or linear projection [ 92 ] , to align features of vision and language models.
Such alignment contains several limitations:
(1) Disparity in parameter scales. The large LLMs [ 48 ] now boosts up to 1000 billion parameters, while the widely-used vision encoders of VLLMs are still around one billion.
This gap may lead to the under-use of LLM’s capacity.
(2) Inconsistent representation. Vision models, trained on pure-vision data or aligned with the BERT series [ 39 , 93 , 70 ] , often exhibit representation inconsistencies with LLMs.
(3) Inefficient connection. The “glue” layers are usually lightweight and randomly initialized, which may not capture the rich cross-modal interactions and dependencies that are crucial for multi-modal understanding and generation. 

These limitations reveal a large gap in both parameter scale and feature representation ability between the vision encoder and the LLM.
To bridge this gap, our inspiration lies in elevating the vision encoder to align with the parameter scale of the LLM and subsequently harmonizing their representations. However, the training of such large-scale models necessitates a vast amount of image-text data obtained from the Internet. The significant heterogeneity and quality variations within this data pose considerable challenges to the training process.
To enhance the efficacy of the training, generative supervision is considered as a complementary approach to contrastive learning, as depicted in Figure 1 . This strategy aims to provide additional guidance to the model during training.
Yet, the suitability of low-quality data for generative training remains a concern.
Besides, how to effectively represent the users’ commands and align the representations between the vision encoder and LLM is another open question. 

To address these issues, we formulate the InternVL, a large-scale vision-language foundation model, which aligns the representation of the scaled-up vision encoder with the LLM and achieves state-of-the-art performance on various visual and vision-language tasks. As shown in Figure 1 (c), InternVL has three key designs: (1) Parameter-balanced vision and language components : It includes a vision encoder scaled up to 6 billion parameters and an LLM middleware with 8 billion parameters, where the middleware functions as a substantial “glue” layer to reorganize visual features based on user commands.
Unlike prior vision-only (Figure 1 (a)) or dual-tower (Figure 1 (b)) structures, our vision encoder and middleware offer flexible combinations for both contrastive and generative tasks.
(2) Consistent representations : To maintain the consistency of representations between the vision encoder and LLM, we employ a pre-trained multilingual LLaMA [ 32 ] , to initialize the middleware and align the vision encoder with it.
(3) Progressive image-text alignment : We leverage image-text data from diverse sources, ensuring training stability through a progressive alignment strategy. This strategy initiates contrastive learning on large-scale noisy image-text data and subsequently transitions to generative learning on fine-grained data. This approach ensures a consistent enhancement of model performance and task scope. 

These designs endow our model with several advantages:
(1) Versatile. It functions as a standalone vision encoder for perception tasks, or collaborates with the language middleware for vision-language tasks and multi-modal dialogue systems. The language middleware bridges the gap between the vision encoder and the LLM decoder.
(2) Strong. By leveraging the training strategy, large-scale parameters, and web-scale data, our model has a powerful representation that helps to achieve state-of-the-art results on various vision and vision-language tasks, as shown in Figure 2 .
(3) LLM-friendly. Due to the aligned feature space with LLMs, our model can smoothly integrate with existing LLMs, such as LLaMA series [ 138 , 139 ] , Vicuna [ 184 ] , and InternLM [ 135 ] . These features distinguish our model from the previous approaches and establish a leading vision-language foundation model for various applications. 

In summary, our contribution has three folds: 

(1) We present a large-scale vision-language foundation model—InternVL, which aligns the large-scale vision encoder with LLMs for the first time. The model demonstrates strong performance on a wide range of generic visual-linguistic tasks, including visual perception tasks, vision-language tasks, and multi-modal dialogue. 

(2) We introduce a progressive image-text alignment strategy for the efficient training of large-scale vision-language foundation models. This strategy maximizes the utilization of web-scale noisy image-text data for contrastive learning and fine-grained, high-quality data for generative learning. 

(3) We extensively compare the proposed model with the current state-of-the-art vision foundation models and VLLMs.
The results indicate that InternVL achieves leading performance on a broad range of generic visual-linguistic tasks, including image classification (ImageNet), semantic segmentation (ADE20K), video classification (Kinetics), image-text retrieval (Flickr30K & COCO), video-text retrieval (MSR-VTT), and image captioning (COCO & Flickr30K & NoCaps). Meanwhile, it is also effective for multi-modal dialogue (MME & POPE & Tiny LVLM). 

2 Related Work 
Figure 3 : The training strategy of the proposed InternVL model. It consists of three progressive stages, including vision-language contrastive training, vision-language generative training, and supervised fine-tuning. These stages effectively leverage public data from diverse sources, ranging from noisy image-text pairs on the web to high-quality caption, VQA, and multi-modal dialogue datasets. 
2.1 Vision Foundation Models 

The past decade has witnessed significant development in foundation models within the field of computer vision. Starting with the pioneering AlexNet [ 73 ] , a variety of convolutional neural networks (CNNs) have emerged, continuously refreshing the ImageNet benchmark [ 57 , 95 , 148 , 160 , 65 , 40 , 33 , 62 ] . In particular, the introduction of residual connections [ 57 ] effectively addressed the problem of vanishing gradients.
This breakthrough led to an era of “big & deep” neural networks,
signifying that, with adequate training and data, larger and deeper models can achieve better performance. In other words, scaling up matters. 

In recent years, ViT [ 42 ] has opened up new possibilities for network architectures in the computer vision field.
ViT and its variants [ 144 , 145 , 178 , 179 , 94 , 37 , 46 , 117 , 25 , 15 ] have significantly increased their capacity and excelled in various important visual tasks.
In the LLM era, these vision foundation models often connect with LLMs through some lightweight “glue” layers [ 92 , 80 , 187 ] .
However, a gap exists as these models primarily derive from visual-only datasets like ImageNet [ 38 ] or JFT [ 173 ] , or are aligned with the BERT series [ 39 , 93 , 70 ] using image-text pairs, lacking direct alignment with LLMs.
Additionally, the prevalent vision models employed to connect with LLMs are still limited to around 1 billion parameters [ 46 , 67 ] , which also constrains the performance of VLLMs. 

2.2 Large Language Models 

Large language models (LLMs) have revolutionized the field of artificial intelligence, enabling natural language processing tasks that were previously thought exclusive to humans [ 153 , 110 , 138 ] . The emergence of GPT-3 [ 153 ] brought a significant leap in capabilities, particularly in few-shot and zero-shot learning, highlighting the immense potential of LLMs. This promise was further realized with the advancements of ChatGPT and GPT-4 [ 110 ] .
The progress in the field has been further accelerated by the emergence of open-source LLMs, including the LLaMA series [ 138 , 139 ] , Vicuna [ 184 ] , InternLM [ 135 ] , MOSS [ 132 ] , ChatGLM [ 44 ] , Qwen [ 4 ] , Baichuan [ 6 ] , and Falcon [ 114 ] , among others [ 134 , 154 , 32 ] .
However, in real scenarios, interactions are not limited to natural language.
The vision modality can bring additional information, which means more possibilities.
Therefore, exploring how to utilize the excellent capabilities of LLMs for multi-modal interactions is poised to become the next research trend. 

2.3 Vision Large Language Models 

Recent advancements have seen the creation of vision large language models (VLLMs) [ 180 , 177 , 181 , 156 , 131 , 3 , 188 , 82 , 75 , 165 , 23 , 79 , 175 , 88 , 168 ] , which aim to enhance language models with the capability to process and interpret visual information. Flamingo [ 3 ] uses the visual and language inputs as prompts and shows remarkable few-shot performance for visual question answering.
Subsequently, GPT-4 [ 110 ] , LLaVA series [ 92 , 100 , 91 ] and MiniGPT-4 [ 187 ] have brought in visual instruction tuning, to improve the instruction-following ability of VLLMs.
Concurrently, models such as VisionLLM [ 147 ] , KOSMOS-2 [ 115 ] , and Qwen-VL et al . [ 5 , 149 , 21 ] have improved VLLMs with visual grounding capabilities, facilitating tasks such as region description and localization.
Many API-based methods [ 96 , 155 , 125 , 166 , 133 , 163 , 97 ] have also attempted to integrate vision APIs with LLMs for solving vision-centric tasks.
Additionally, PaLM-E [ 43 ] and EmbodiedGPT [ 108 ] represent advanced efforts in adapting VLLMs for embodied applications, significantly expanding their potential applications.
These works showcase that VLLMs have achieved significant breakthroughs.
However, the progress of vision and vision-language foundation models, equally essential for VLLMs, has not kept pace. 

3 Proposed Method 

3.1 Overall Architecture 

As depicted in Figure 3 , unlike traditional vision-only backbones [ 57 , 94 , 148 ] and dual-encoder models [ 117 , 67 , 130 ] , the proposed InternVL is designed with a vision encoder InternViT-6B and a language middleware QLLaMA.
Specifically, InternViT-6B is a vision transformer with 6 billion parameters, customized to achieve a favorable trade-off between performance and efficiency.
QLLaMA is a language middleware with 8 billion parameters, initialized with a multilingual-enhanced LLaMA [ 32 ] . It could provide robust multilingual representation for image-text contrastive learning, or serve as a bridge to connect the vision encoder and the off-the-shelf LLM decoder. 

To align the two large-scale components with substantial gaps in modalities and structures,
we introduce a progressive alignment training strategy.
The training strategy is conducted progressively, beginning with contrastive learning on large-scale noisy data, and gradually moving towards generative learning on exquisite and high-quality data.
In this way, we ensure the effective organization and full utilization of web-scale image-text data from a variety of sources.
Then, equipped with the aligned vision encoder and language middleware, our model functions like a Swiss Army knife.
It boasts a flexible composition that can be adapted for a wide array of generic visual-linguistic tasks. These tasks range from visual perception and image/video-text retrieval to image captioning, visual question answering, and multi-modal dialogue, among others. 

name width depth MLP #heads #param (M) 

ViT-G [ 173 ] 1664 48 8192 16 1843 

ViT-e [ 23 ] 1792 56 15360 16 3926 

EVA-02-ViT-E [ 130 ] 1792 64 15360 16 4400 

ViT-6.5B [ 128 ] 4096 32 16384 32 6440 

ViT-22B [ 37 ] 6144 48 24576 48 21743 

InternViT-6B (ours) 3200 48 12800 25 5903 
Table 1 : Architecture details of the InternViT-6B model. Figure 4 : Different ways to use InternVL. By flexibly combining the vision encoder and the language middleware, InternVL can support various vision-language tasks, including contrastive tasks, generative tasks, and multi-modal dialogue. 
3.2 Model Design 

Large-Scale Vision Encoder: InternViT-6B. We implement the vision encoder of InternVL with vanilla vision transformer (ViT) [ 42 ] .
To match the scale of LLMs, we scale up the vision encoder to 6 billion parameters, resulting in the InternViT-6B model.
To obtain a good trade-off between accuracy, speed, and stability, we conduct a hyperparameter search for InternViT-6B.
We vary the model depth within {32, 48, 64, 80}, the head dimension within {64, 128}, and the MLP ratio within {4, 8}. The model width and the head number are calculated based on the given model scale and other hyperparameters. 

We employ contrastive learning on a 100M subset of the LAION-en dataset [ 120 ] to measure the accuracy, speed, and stability of InternViT-6B variants with different configurations.
We report the following findings: (1) Speed. For different model settings, when computation is not saturated, the models with smaller depths exhibit faster speed per image. However, as the GPU computation is fully utilized, the speed difference becomes negligible;
(2) Accuracy. With the same number of parameters, the depth, head dimension, and MLP ratio have little impact on the performance. Based on these findings, we identified the most stable configuration for our final model, as shown in Table 1 . 

Language Middleware: QLLaMA. The language middleware QLLaMA is proposed to align visual and linguistic features. As shown in Figure 3 , QLLaMA is developed based on the pre-trained multilingual LLaMA [ 32 ] , and newly added 96 learnable queries and cross-attention layers (1 billion parameters) that are randomly initialized.
This manner allows QLLaMA to smoothly integrate visual elements into the language model, thereby enhancing the coherence and effectiveness of the combined features. 

Compared to recently popular approaches [ 81 , 92 ] that use lightweight “glue” layers, such as QFormer [ 81 ] and linear layers [ 92 ] to connect vision encoder and LLMs,
our method has three advantages: (1) By initializing with the pre-trained weights of [ 32 ] , QLLaMA can transform image tokens generated by InternViT-6B into the representation that is aligned with the LLMs; (2) QLLaMA has 8 billion parameters for vision-language alignment, which are 42 times larger than the QFormer. Therefore, even with a frozen LLM decoder, InternVL can achieve promising performance on multi-modal dialogue tasks.
(3) It can also be applied to contrastive learning, providing a powerful text representation for image-text alignment tasks, such as zero-shot image classification and image-text retrieval. 

“Swiss Army Knife” Model: InternVL. By flexibly combining the vision encoder and the language middleware, InternVL can support various vision or vision-language tasks. 

(1) For visual perception tasks , the vision encoder of InternVL, i.e . InternViT-6B, can be used as the backbone for vision tasks.
Given an input image I ∈ ℝ H × W × 3 I\in\mathbb{R}^{H\times W\times 3} , our model can generate a feature map F ∈ ℝ H / 14 × W / 14 × D F\in\mathbb{R}^{H/14\times W/14\times D} for dense prediction tasks, or work with global average pooling and linear projection to make image classification. 

characteristics stage 1 stage 2 

dataset language original cleaned remain cleaned remain 

LAION-en [ 120 ] English 2.3B 1.94B 84.3% 91M 4.0% 

LAION-COCO [ 121 ] 663M 550M 83.0% 550M 83.0% 

COYO [ 14 ] 747M 535M 71.6% 200M 26.8% 

CC12M [ 20 ] 12.4M 11.1M 89.5% 11.1M 89.5% 

CC3M [ 124 ] 3.0M 2.6M 86.7% 2.6M 86.7% 

SBU [ 112 ] 1.0M 1.0M 100% 1.0M 100% 

Wukong [ 55 ] Chinese 100M 69.4M 69.4% 69.4M 69.4% 

LAION-multi [ 120 ] Multi 2.2B 1.87B 85.0% 100M 4.5% 

Total Multi 6.03B 4.98B 82.6% 1.03B 17.0% 
Table 2 : Details of the training data for InternVL in stage 1 and stage 2. Among them, LAION-en [ 120 ] , LAION-multi [ 120 ] , COYO [ 14 ] , and Wukong [ 55 ] are web-scale image-text pairs data.
LAION-COCO [ 121 ] is a synthetic dataset with high-quality captions from LAION-en. CC12M [ 20 ] , CC3M [ 124 ] , SBU [ 112 ] are academic caption datasets. “Multi” means multilingual. 

(2) For contrastive tasks , as shown in Figure 4 (a) (b),
we introduce two inference modes: InternVL-C and InternVL-G , using the vision encoder or the combination of InternViT and QLLaMA to encode visual features.
Specifically, we apply attention pooling to the visual features of InternViT or the query features of QLLaMA, to calculate the global visual feature I f I_{f} .
Besides, we encode text as T f T_{f} by extracting the feature from the [EOS] token of QLLaMA.
By computing similarity scores between I f I_{f} and T f T_{f} , we support various contrastive tasks such as image-text retrieval. 

(3) For generative tasks , unlike QFormer [ 80 ] , QLLaMA inherently has promising image captioning abilities thanks to its scaled-up parameters.
The queries of QLLaMA reorganize the visual representations from InternViT-6B and play as the prefix texts for QLLaMA. The subsequent text tokens are generated one by one sequentially. 

(4) For multi-modal dialogue , we introduce InternVL-Chat , leveraging InternVL as the visual component to connect with LLMs.
For this purpose, we have two distinct configurations.
One option is to employ the InternViT-6B independently, as shown in Figure 4 (c).
The alternative is to employ the complete InternVL model concurrently, as illustrated in Figure 4 (d). 

task #samples dataset 

Captioning 588K COCO Caption [ 22 ] , TextCaps [ 126 ] 

VQAv2 [ 54 ] , OKVQA [ 104 ] , A-OKVQA [ 122 ] , 

VQA 1.1M IconQA [ 99 ] , AI2D [ 71 ] , GQA [ 64 ] 

OCR-VQA [ 107 ] , ChartQA [ 105 ] , DocVQA [ 29 ] , 

ST-VQA [ 12 ] , EST-VQA [ 150 ] , InfoVQA [ 106 ] , 

OCR 294K LLaVAR [ 182 ] 

Grounding 323K RefCOCO/+/g [ 170 , 103 ] , Toloka [ 140 ] 

Grounded Cap. 284K RefCOCO/+/g [ 170 , 103 ] 

LLaVA-150K [ 92 ] , SVIT [ 183 ] , VisDial [ 36 ] , 

Conversation 1.4M LRV-Instruction [ 90 ] , LLaVA-Mix-665K [ 91 ] 
Table 3 : Details of the training data for InternVL in stage 3. We collect a wide range of high-quality instruction data, totaling approximately 4 million samples. For a fair comparison, we only use the training split of these datasets. 
3.3 Alignment Strategy 

As shown in Figure 3 , the training of InternVL consists of three progressive stages, including vision-language contrastive training, vision-language generative training, and supervised fine-tuning. These stages effectively leverage public data from diverse sources, ranging from noisy image-text pairs on the web to high-quality caption, VQA, and multi-modal dialogue datasets. 

Vision-Language Contrastive Training. In the first stage, we conduct contrastive learning to align InternViT-6B with a multilingual LLaMA-7B [ 32 ] on web-scale, noisy image-text pairs.
The data are all publicly available and comprise multilingual content, including LAION-en [ 120 ] , LAION-multi [ 120 ] , LAION-COCO [ 121 ] , COYO [ 14 ] , Wukong [ 55 ] , etc.
We use the combination of these datasets and filter out some extremely low-quality data to train our model.
As summarized in Table 2 , the original dataset contains 6.03 billion image-text pairs, and 4.98 billion remains after cleaning.
More details about data preparation will be provided in the supplementary materials. 

During training, we adopt the LLaMA-7B to encode the text as T f T_{f} , and use InternViT-6B to extract the visual feature I f I_{f} . Following the objective function of CLIP [ 117 ] , we minimize a symmetric cross-entropy loss on the similarity scores of image-text pairs in a batch. This stage allows InternVL to excel on contrastive tasks like zero-shot image classification and image-text retrieval, and the vision encoder of this stage can also perform well on visual perception tasks like semantic segmentation. 

Vision-Language Generative Training .
In the second stage of training, we connect InternViT-6B with QLLaMA and adopt a generative training strategy.
Specifically, QLLaMA inherits the weights of LLaMA-7B in the first stage.
We keep both InternViT-6B and QLLaMA frozen and only train the newly added learnable queries and cross-attention layers with filtered, high-quality data.
Table 2 summarizes the datasets for the second stage.
It can be seen that we further filtered out data with low-quality captions, reducing it from 4.98 billion in the first stage to 1.03 billion. 

Following the loss function of BLIP-2 [ 81 ] , the loss in this stage is computed as the sum of three components: image-text contrastive (ITC) loss, image-text matching (ITM) loss, and image-grounded text generation (ITG) loss.
This enables the queries to extract powerful visual representations, and further align feature space with LLMs, attributable to the effective training objectives and the utilization of our large-scale, LLM-initialized QLLaMA. 

method #param IN-1K IN-ReaL IN-V2 IN-A IN-R IN-Ske avg. 

OpenCLIP-H [ 67 ] 0.6B 84.4 88.4 75.5 − - − - − - − - 

OpenCLIP-G [ 67 ] 1.8B 86.2 89.4 77.2 63.8 87.8 66.4 78.5 

DINOv2-g [ 111 ] 1.1B 86.5 89.6 78.4 75.9 78.8 62.5 78.6 

EVA-01-CLIP-g [ 46 ] 1.1B 86.5 89.3 77.4 70.5 87.7 63.1 79.1 

MAWS-ViT-6.5B [ 128 ] 6.5B 87.8 – – – – – – 

ViT-22B ∗ [ 37 ]
