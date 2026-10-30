标题：InternVL 2.5: Expanding Performance Boundaries of Open-Source Multimodal Models
机构：OpenGVLab, Shanghai AI Laboratory（上海人工智能实验室）
arXiv：2412.05271
来源：https://arxiv.org/abs/2412.05271
说明：以下摘自 arXiv 公开 HTML 正文（自摘要起，去掉页面装饰与文末参考文献），供工作区问答测试。

Abstract 
We introduce InternVL 2.5, an advanced multimodal large language model (MLLM) series that builds upon InternVL 2.0, maintaining its core model architecture while introducing significant enhancements in training and testing strategies as well as data quality.
In this work, we delve into the relationship between model scaling and performance, systematically exploring the performance trends in vision encoders, language models, dataset sizes, and test-time configurations.
Through extensive evaluations on a wide range of benchmarks, including multi-discipline reasoning, document understanding, multi-image / video understanding, real-world comprehension, multimodal hallucination detection, visual grounding, multilingual capabilities, and pure language processing, InternVL 2.5 exhibits competitive performance, rivaling leading commercial models such as GPT-4o and Claude-3.5-Sonnet. Notably, our model is the first open-source MLLMs to surpass 70% on the MMMU benchmark, achieving a 3.7-point improvement through Chain-of-Thought (CoT) reasoning and showcasing strong potential for test-time scaling. HuggingFace demo see https://huggingface.co/spaces/OpenGVLab/InternVL 

† † * equal contribution; † \dagger interns at OpenGVLab, Shanghai AI Laboratory; 
🖂 corresponding authors (daijifeng@tsinghua.edu.cn, wangwenhai@pjlab.org.cn). 
1 Introduction 

In recent years, multimodal large language models (MLLMs) [ 60 , 137 , 246 , 36 , 35 , 248 , 140 , 228 , 192 , 275 , 143 , 54 , 170 ] have emerged as a pivotal technology in artificial intelligence, capable of processing and understanding information from multiple modalities such as text, images, and videos. These models promise breakthroughs across fields like natural language processing, computer vision, and human-computer interaction. However, developing large-scale MLLMs remains a challenging task, requiring significant computational resources, sophisticated architectures, and the ability to effectively integrate diverse data types in a scalable manner. 

Various attempts have been made to address these challenges, including enhancing model architectures [ 220 , 232 , 5 , 172 , 157 , 210 ] , scaling vision encoders [ 252 , 66 , 36 , 293 , 185 ] and language models [ 231 , 235 , 64 , 19 , 229 , 221 , 62 ] , incorporating more diverse and high-quality datasets [ 124 , 234 , 25 , 155 ] , and refining the test-time scaling process [ 215 , 249 , 230 ] to boost performance. Notable commercial models, like GPT-4o [ 192 ] and Claude-3.5-Sonnet [ 8 ] , have demonstrated exceptional performance, their closed nature limits transparency and accessibility, leaving gaps in the open-source community.
While open-source multimodal models such as the InternVL series [ 36 , 35 , 71 ] and Qwen-VL series [ 13 , 246 ] have provided high-performance, transparent alternatives, they still fall short in terms of achieving the desired levels of performance and efficiency. 

Figure 1 : Performance of various MLLMs on the OpenCompass leaderboard. InternVL 2.5 showcases strong multimodal capabilities, rivaling closed-source models like GPT-4o [ 192 ] and Claude-3.5-Sonnet [ 8 ] .
However, since the OpenCompass score is derived from 8 academic VQA benchmarks and covers only a subset of overall capabilities, we still need further effort to match the performance with closed-source models. 

In this work, we introduce InternVL 2.5, an advanced large-scale MLLM series that builds upon the foundational architecture of InternVL 2.0. Continuing the objectives of the entire InternVL series, we aim to bridge the performance gap between commercial closed-source models and open-source multimodal models.
In InternVL 2.5, we systematically explore various factors in MLLM, including how changes in vision encoders, language models, dataset sizes, and inference times affect the overall performance of the model, demonstrating the relationship between scaling and performance in multimodal models. Specifically, we have some interesting findings:
(1) Large vision encoders significantly reduce the dependency on training data when scaling up MLLMs. As shown in Table 3 , compared to Qwen2-VL-72B [ 246 ] equipped with a 600M vision encoder, our InternVL2.5-78B with a 6B vision encoder can achieve better performance using only 1/10 of the training tokens. This greatly reduces the exploration cost when scaling up MLLMs;
(2) Data quality matters. Upgrading InternVL from 2.0 to 2.5 doubled the dataset size, but strict filtering greatly improved quality. For example, we carefully excluded the anomalous samples ( e.g. , repetitive patterns), achieving substantial improvements in Chain-of-Thought (CoT) reasoning tasks such as MMMU [ 289 ] and complex challenges like the OlympiadBench [ 80 ] . Note that, most existing open-source MLLMs tend to underperform when using CoT [ 249 ] .
(3) Test-time scaling is beneficial for difficult multimodal QA. For challenging tasks such as MMMU, the InternVL2.5-78B with CoT reaches 70.1%, which is 3.7 points higher than the direct response. Subsequently, we have successfully verified that CoT can be further combined with majority voting and bring additional improvements. 

Our contributions can be summarized as follows: 

(1) We release InternVL 2.5 to the open-source community, providing a powerful tool for the development and application of multimodal AI systems and encouraging further research in this domain. 

(2) We investigate how scaling different components of the MLLMs such as vision encoders, language models, dataset sizes, and inference time affect performance. 

(3) Through extensive evaluations on diverse benchmarks—including multi-discipline reasoning, document understanding, multi-image / video understanding, real-world comprehension, multimodal hallucination detection, visual grounding, multilingual capabilities, and pure language processing—InternVL 2.5 exhibits competitive performance, rivaling leading commercial models like GPT-4o [ 192 ] and Claude-3.5-Sonnet [ 8 ] . It is the first open-source MLLM to surpass 70% on the MMMU validation set [ 289 ] , setting a new benchmark and highlighting the potential of open-source solutions in advancing multimodal AI. 

2 Model Architecture 
Figure 2 : Overall architecture. InternVL 2.5 retains the same model architecture as InternVL 1.5 [ 35 ] and InternVL 2.0, i.e. the widely-used “ViT-MLP-LLM” paradigm, which combines a pre-trained InternViT-300M or InternViT-6B with LLMs [ 19 , 229 ] of various sizes via an MLP projector. Consistent with previous versions, we apply a pixel unshuffle operation to reduce the 1024 visual tokens produced by each 448 × \times 448 image tile to 256 tokens.
Moreover, compared to InternVL 1.5, InternVL 2.0 and 2.5 introduced additional data types, incorporating multi-image and video data alongside the existing single-image and text-only data. 
2.1 Overall Architecture 

As shown in Figure 2 and Table 2 , InternVL 2.5 retains the same model architecture as its predecessors, InternVL 1.5 [ 35 ] and InternVL 2.0, following the “ViT-MLP-LLM” paradigm widely adopted in various MLLM studies [ 150 , 151 , 36 , 316 , 162 , 246 , 124 , 256 ] . 

In this new version, our implementation of this architecture integrates a newly incrementally pre-trained InternViT-6B or InternViT-300M with various pre-trained LLMs of different sizes and types, including InternLM 2.5 [ 19 ] and Qwen 2.5 [ 229 ] , using a randomly initialized 2-layer MLP projector. As in the previous version, to enhance scalability for high-resolution processing, we simply applied a pixel unshuffle operation, reducing the number of visual tokens to one-quarter of the original. Consequently, in our model, a 448 × \times 448 image tile is represented by 256 visual tokens. 

In terms of input data preprocessing, we adopted a similar dynamic resolution strategy as InternVL 1.5, dividing images into tiles of 448 × \times 448 pixels based on the aspect ratio and resolution of the input images. The key difference, starting from InternVL 2.0, is that we additionally introduced support for multi-image and video data, as shown in Figure 2 (b). Different data types correspond to different preprocessing configurations, which we will detail in Section 3.1 . 

2.2 Vision Encoder 

InternVL employs InternViT [ 36 ] as the vision encoder. To better document the training progression of InternViT, we have provided detailed information in Table 1 . InternViT currently has two different model sizes, including InternViT-6B and InternViT-300M. 

InternViT-6B. InternViT-6B-224px was first introduced in our CVPR paper [ 36 ] , and its structure follows the vanilla ViT [ 61 ] , with minor adjustments incorporating QK-Norm [ 53 ] and RMSNorm [ 294 ] .
It had 5.9B parameters, 48 layers, a hidden size of 3200, and 25 heads, and it was trained using a contrastive loss [ 195 ] .
Due to the limited gains at that time, we adopted an incremental pre-training strategy to continuously refine its weights. Specifically, we connected InternViT-6B to an LLM via an MLP projector and, following a brief MLP warmup, jointly trained the InternViT-6B using a next token prediction loss (as shown in Figure 4 (a)) to enhance its visual feature extraction capabilities.
In the V1.0 and V1.2 versions, we used a fixed resolution of 448 × \times 448 for training, but in later versions, we switched to dynamic resolution training to improve high-resolution processing.
As detailed in the InternVL 1.5 report [ 35 ] , we removed the last three layers of InternViT-6B-448px-V1.2 , reducing its depth from 48 to 45 layers, as these layers were more tuned to the CLIP loss objective, prioritizing global alignment over local information.
As a result, all subsequent versions, including the latest InternViT-6B-448px-V2.5 , have 45 layers and 5.5B parameters. 

InternViT-300M. InternViT-300M-448px-Distill is a distilled variant of the teacher model, InternViT-6B-448px-V1.5 , utilizing a cosine distillation loss.
This model comprises 0.3B parameters, 24 layers, a hidden size of 1024, and 16 attention heads. Unlike the 6B version, the 0.3B variant employs standard LayerNorm [ 11 ] without QK-Norm [ 53 ] .
To reduce distillation costs, we initialized this model using CLIP-ViT-Large-336px [ 195 ] where applicable, despite some architectural differences.
After distillation, we integrated this model with an LLM and, following a similar procedure as described above, trained the vision encoder with dynamic high-resolution and the NTP loss.
Then, we extracted the vision encoder and released it as InternViT-300M-448px .
In this report, we further refined the InternViT-300M by incrementally pre-training the previous weights on a more diverse data mixture using the NTP loss, leading to the enhanced InternViT-300M-448px-V2.5 . 

Model Name Train Res. Width Depth MLP #Heads QK-Norm Norm Type Loss Type #Param 

InternViT-6B-224px fixed 224 3200 48 12800 25 ✓ RMS CLIP 5.9B 

InternViT-6B-448px-V1.0 fixed 448 3200 48 12800 25 ✓ RMS NTP 5.9B 

InternViT-6B-448px-V1.2 fixed 448 3200 45 12800 25 ✓ RMS NTP 5.5B 

InternViT-6B-448px-V1.5 dynamic 448 3200 45 12800 25 ✓ RMS NTP 5.5B 

InternViT-6B-448px-V2.5 dynamic 448 3200 45 12800 25 ✓ RMS NTP 5.5B 

InternViT-300M-448px-Distill fixed 448 1024 24 4096 16 ✗ LN Cosine 0.3B 

InternViT-300M-448px dynamic 448 1024 24 4096 16 ✗ LN NTP 0.3B 

InternViT-300M-448px-V2.5 dynamic 448 1024 24 4096 16 ✗ LN NTP 0.3B 
Table 1 : Details of InternViT-6B and InternViT-300M models. “fixed 224” refers to training images resized to 224 × \times 224, while “dynamic 448” means the model is trained with dynamic high resolution, with each image tile being 448 × \times 448.
“CLIP” refers to the contrastive loss, “Cosine” represents the cosine distillation loss, while “NTP” indicates the next token prediction loss. 
Model Name #Param Vision Encoder Language Model OpenCompass 

InternVL-Chat-V1.5 25.5B InternViT-6B-448px-V1.5 internlm2-chat-20b 61.7 

InternVL2-1B 0.9B InternViT-300M-448px Qwen2-0.5B-Instruct 48.3 

InternVL2-2B 2.2B InternViT-300M-448px internlm2-chat-1.8b 54.0 

InternVL2-4B 4.2B InternViT-300M-448px Phi-3-mini-128k-instruct 60.6 

InternVL2-8B 8.1B InternViT-300M-448px internlm2_5-7b-chat 64.1 

InternVL2-26B 25.5B InternViT-6B-448px-V1.5 internlm2-chat-20b 66.4 

InternVL2-40B 40.1B InternViT-6B-448px-V1.5 Nous-Hermes-2-Yi-34B 69.7 

InternVL2-Llama3-76B 76.3B InternViT-6B-448px-V1.5 Hermes-2-Theta-Llama-3-70B 71.0 

InternVL2.5-1B 0.9B InternViT-300M-448px-V2.5 Qwen2.5-0.5B-Instruct 54.5 

InternVL2.5-2B 2.2B InternViT-300M-448px-V2.5 internlm2_5-1_8b-chat 59.8 

InternVL2.5-4B 3.7B InternViT-300M-448px-V2.5 Qwen2.5-3B-Instruct 65.1 

InternVL2.5-8B 8.1B InternViT-300M-448px-V2.5 internlm2_5-7b-chat 68.1 

InternVL2.5-26B 25.5B InternViT-6B-448px-V2.5 internlm2_5-20b-chat 71.3 

InternVL2.5-38B 38.4B InternViT-6B-448px-V2.5 Qwen2.5-32B-Instruct 73.3 

InternVL2.5-78B 78.4B InternViT-6B-448px-V2.5 Qwen2.5-72B-Instruct 75.5 

InternVL2.5-Pro – InternViT-6B-448px-V2.5 – – 
Table 2 : Pre-trained models used in the InternVL series. In the InternVL 2.5 series, we upgraded both the vision encoder and the language model, resulting in improved performance.
The OpenCompass scores for InternVL 1.5 and InternVL 2.0 were collected from the OpenCompass leaderboard, while the scores for InternVL 2.5 series were obtained through our local testing. 
2.3 Large Language Model 

In Table 2 , we provide an overview of the language models used across different versions of InternVL, including InternVL 1.5, InternVL 2.0, and the latest InternVL 2.5. As shown, earlier versions primarily built on language models such as InternLM 2 [ 19 ] , Qwen 2 [ 268 ] , Phi 3 [ 1 ] , Yi [ 279 ] , and Llama 3 [ 64 ] . To achieve better performance, in the InternVL 2.5 series, we have comprehensively upgraded the language backbones to the latest state-of-the-art models, including InternLM 2.5 [ 19 ] and Qwen 2.5 [ 229 ] . 

Figure 3 : Illustration of the data formats for various data types. (a) For single-image datasets, the maximum number of tiles n max n_{\text{max}} is allocated to a single image, ensuring maximum resolution for the input.
(b) For multi-image datasets, the total number of tiles n max n_{\text{max}} is distributed proportionally across all images within the sample.
(c) For video datasets, the method simplifies the approach by setting n max = 1 n_{\text{max}}=1 , resizing individual frames to a fixed resolution of 448 × \times 448. 
3 Training Strategy 

3.1 Dynamic High-Resolution for Multimodal Data 

In InternVL 2.0 and 2.5, we extend the dynamic high-resolution training approach introduced in InternVL 1.5 [ 35 ] , enhancing its capabilities to handle multi-image and video datasets.
The process mainly consists of the following steps: 

Closest Aspect Ratio Matching. Given an input image I I with dimensions W × H W\times H , the aspect ratio is computed as r = W H r=\frac{W}{H} .
The objective is to resize the image into tiles of size S × S S\times S (where S = 448 S=448 ) while selecting the closest aspect ratio that minimizes distortion.
The number of tiles, n tiles n_{\text{tiles}} , is constrained within a predefined range [ n min , n max ] [n_{\min},n_{\max}] . 

To find the optimal aspect ratio for resizing, we define the set of target aspect ratios ℛ \mathcal{R} as: 

ℛ = { i / j ∣ 1 ≤ i , j ≤ n , i × j ∈ [ n min , n max ] } . \mathcal{R}=\left\{i/j\mid 1\leq i,j\leq n,\,i\times j\in[n_{\min},n_{\max}]\right\}.\vskip-3.00003pt (1) 

The closest aspect ratio r best r_{\text{best}} is selected by minimizing the difference between the original aspect ratio r r and each target aspect ratio r target r_{\text{target}} : 

r best = arg ⁡ min r target ∈ ℛ ​ | r − r target | . r_{\text{best}}=\arg\min_{r_{\text{target}}\in\mathcal{R}}\left|r-r_{\text{target}}\right|.\vskip-3.00003pt (2) 

In cases where multiple aspect ratios produce the same difference ( e.g. , 1:2 and 2:4), we prioritize the aspect ratio that results in an area less than or equal to twice the original image size.
This helps to some extent in preventing the excessive enlargement of low-resolution images. 

Image Resizing and Splitting. Once the best aspect ratio is determined, the image is resized to new dimensions W new × H new W_{\text{new}}\times H_{\text{new}} , where i best i_{\text{best}} and j best j_{\text{best}} are the factors corresponding to r best r_{\text{best}} : 

W new = S × i best , H new = S × j best . W_{\text{new}}=S\times i_{\text{best}},\quad H_{\text{new}}=S\times j_{\text{best}}.\vskip-3.00003pt (3) 

The image is then split into tiles of size S × S S\times S , with the number of tiles calculated as n tiles = i best × j best n_{\text{tiles}}=i_{\text{best}}\times j_{\text{best}} . Each tile is cropped from the resized image to ensure consistent size. 

Thumbnail Generation. Optionally, if the number of tiles n tiles > 1 n_{\text{tiles}}>1 , the original image I I is resized to a square of dimensions S × S S\times S to generate an additional thumbnail I thumb I_{\text{thumb}} .
This thumbnail is appended to the list of tiles, providing a global view alongside the localized tiles.
In cases where n tiles = 1 n_{\text{tiles}}=1 , there is no thumbnail to append, and the mechanism naturally skips this step. 

Data Formats for Different Data Types. As shown in Figure 3 , the dynamic high-resolution method in InternVL 2.0 and 2.5 extends beyond single-image datasets to also support multi-image and video datasets. 

For single-image datasets, the maximum number of tiles n max n_{\text{max}} is allocated to a single image, ensuring that it is processed at the highest possible resolution. In this scenario, visual tokens are enclosed within <img> and </img> tags, with no additional auxiliary tags used. 

In the case of multi-image datasets, the total number of tiles n max n_{\text{max}} is distributed across all images within one sample. Each image is identified by an auxiliary tag like Image-1 to clearly label individual images. The images themselves are enclosed within <img> and </img> tags, denoting the start and end of the image data. The number of tiles assigned to each image I i I_{i} is proportional to the total number of images N image N_{\text{image}} , following the equation: 

n max, i = max ⁡ ( 1 , ⌊ n max N image ⌋ ) . n_{\text{max, i}}=\max\left(1,\left\lfloor\frac{n_{\text{max}}}{N_{\text{image}}}\right\rfloor\right).\vskip-3.00003pt (4) 

For video data, this approach is simplified by setting n max = 1 n_{\text{max}}=1 . Each video frame is resized to a fixed resolution of 448 × \times 448, eliminating the need for tiling. This is because, during training, a large number of frames ( e.g. , 32 or 64) are typically extracted from a single video. For our model, even without high-resolution input, this still results in 8,192 or 16,384 visual tokens.
Each video frame, labeled with tags like Frame-1 , is enclosed within the <img> and </img> tags, similar to image data. 

3.2 Single Model Training Pipeline 
Figure 4 : Illustration of the training pipeline and progressive scaling strategy. (a) Single model training pipeline. The training process is divided into three stages—Stage 1 (MLP warmup), optional Stage 1.5 (ViT incremental learning), and Stage 2 (full model instruction tuning).
The multi-stage design progressively enhances vision-language alignment, stabilizes training, and prepares modules for integration with larger LLMs.
(b) Progressive scaling strategy. The ViT module trained with a smaller LLM in earlier stages can be easily integrated with larger LLMs, enabling scalable model alignment with affordable resource overhead. 

Settings InternVL2.5-1B InternVL2.5-2B InternVL2.5-4B InternVL2.5-8B 

Stage 1 Stage 2 Stage 1 Stage 2 Stage 1 Stage 2 Stage 1 Stage 1.5 Stage 2 

Dataset Pre-train Mixture Fine-tune Mixture Pre-train Mixture Fine-tune Mixture Pre-train Mixture Fine-tune Mixture Pre-train Mixture Pre-train Mixture Fine-tune Mixture 

Trainable MLP Full Model MLP Full Model MLP Full Model MLP ViT+MLP Full Model 

Packed Batch Size 512 512 512 512 512 512 512 1024 512 

Learning Rate 2e-4 4e-5 2e-4 4e-5 2e-4 4e-5 2e-4 1e-5 4e-5 

Context Length 16384 16384 16384 16384 16384 16384 16384 16384 16384 

Image Tile Threshold 48 48 48 48 48 48 48 48 48 

ViT Drop Path 0.0 0.1 0.0 0.1 0.0 0.1 0.0 0.1 0.1 

Weight Decay 0.01 0.01 0.01 0.01 0.01 0.01 0.05 0.05 0.05 

Training Epochs – 4 – 4 – 2 – – 1 

Training Tokens ∼ \sim 191B ∼ \sim 176B ∼ \sim 277B ∼ \sim 176B ∼ \sim 164B ∼ \sim 88B ∼ \sim 22B ∼ \sim 76B ∼ \sim 44B 

Settings InternVL2.5-26B InternVL2.5-38B InternVL2.5-78B 

Stage 1 Stage 1.5 Stage 2 Stage 1 Stage 2 Stage 1 Stage 2 

Dataset Pre-train Mixture Pre-train Mixture Fine-tune Mixture Pre-train Mixture Fine-tune Mixture Pre-train Mixture Fine-tune Mixture 

Trainable MLP ViT+MLP Full Model MLP Full Model MLP Full Model 

Packed Batch Size 512 1024 512 512 512 512 512 

Learning Rate 2e-4 1e-5 2e-5 2e-4 2e-5 2e-4 2e-5 

Context Length 16384 16384 16384 16384 16384 16384 16384 

Image Tile Threshold 48 48 48 48 48 48 48 

ViT Drop Path 0.0 0.4 0.4 0.0 0.4 0.0 0.4 

Weight Decay 0.05 0.05 0.05 0.05 0.05 0.05 0.05 

Training Epochs – – 1 – 1 – 1 

Training Tokens ∼ \sim 31B ∼ \sim 146B ∼ \sim 44B ∼ \sim 107B ∼ \sim 44B ∼ \sim 76B ∼ \sim 44B 

Table 3 : Training configurations and hyperparameters for InternVL 2.5. This table presents the training setups for various scales of InternVL 2.5 models.
The configurations are carefully optimized to ensure efficient scaling and performance across different parameter sizes and training stages.
Notably, Qwen2-VL [ 246 ] processed a cumulative total of 1.4T tokens, while our InternVL2.5-78B is trained on just ∼ \sim 120B tokens. 

The training pipeline for a single model in InternVL 2.5 is structured across three stages, designed to enhance the model’s visual perception and multimodal capabilities. Each stage progressively integrates vision and language modalities, balancing performance optimization with training efficiency. 

Stage 1: MLP Warmup. As shown in Figure 4 (a), the training begins with warming up the MLP projector, which is the initial bridge between visual and language representations. In this stage, only the MLP projector is trained while both the vision encoder ( i.e. , InternViT [ 36 ] ) and language model are frozen.
To achieve optimal performance, we begin using the dynamic high-resolution training strategy from this stage, even though it increases the training cost. 

In this phase, we utilize the pre-training data mixture as outlined in Table 4 . The data is formatted in a structured ChatML style and optimized with the NTP loss.
Additionally, a higher learning rate is applied to accelerate convergence, allowing the MLP to quickly adapt to the LLM’s input space and establish robust cross-modal alignment.
The MLP warmup phase ensures the model is well-prepared to handle multimodal tasks before unlocking additional trainable components in later stages, thereby improving training stability. 

Stage 1.5: ViT Incremental Learning (Optional). As shown in Figure 4 (a), Stage 1.5 introduces incremental learning for the vision encoder. During this stage, both the vision encoder and MLP projector are trainable, and training is conducted using the same pre-training data mixture and NTP loss as in Stage 1.
The aim of this stage is to enhance the vision encoder’s ability to extract visual features, allowing it to capture more comprehensive information, especially for domains that are relatively rare in web-scale datasets ( e.g. , LAION-5B [ 203 ] ), such as multilingual OCR data and mathematical charts, among others. 

As shown in Table 3 , a lower learning rate is used in this stage to prevent catastrophic forgetting, ensuring the encoder doesn’t lose previously learned capabilities. Additionally, the vision encoder only needs to be trained once unless new domain requirements or data are introduce
