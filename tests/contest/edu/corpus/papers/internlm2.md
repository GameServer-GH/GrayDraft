标题：InternLM2 Technical Report
机构：Shanghai AI Laboratory（上海人工智能实验室）
arXiv：2403.17297
来源：https://arxiv.org/abs/2403.17297
说明：以下摘自 arXiv 公开 HTML 正文（自摘要起，去掉页面装饰与文末参考文献），供工作区问答测试。

Abstract 
The evolution of Large Language Models (LLMs) like ChatGPT and GPT-4 has sparked discussions on the advent of Artificial General Intelligence (AGI). However, replicating such advancements in open-source models has been challenging. This paper introduces InternLM2, an open-source LLM that outperforms its predecessors in comprehensive evaluations across 6 dimensions and 30 benchmarks, long-context modeling, and open-ended subjective evaluations through innovative pre-training and optimization techniques. The pre-training process of InternLM2 is meticulously detailed, highlighting the preparation of diverse data types including text, code, and long-context data. InternLM2 efficiently captures long-term dependencies, initially trained on 4k tokens before advancing to 32k tokens in pre-training and fine-tuning stages, exhibiting remarkable performance on the 200k “Needle-in-a-Haystack” test. InternLM2 is further aligned using Supervised Fine-Tuning (SFT) and a novel Conditional Online Reinforcement Learning from Human Feedback (COOL RLHF) strategy that addresses conflicting human preferences and reward hacking. By releasing InternLM2 models in different training stages and model sizes, we provide the community with insights into the model’s evolution. 

1 Introduction 

Since the introduction of ChatGPT and GPT-4 ( OpenAI, 2023 ) , Large Language Models (LLMs) have surged in popularity across the academic and industrial spheres. Models trained on billions of tokens have demonstrated profound empathy and problem-solving capabilities, leading to widespread speculation that the era of Artificial General Intelligence (AGI) may soon be upon us. Despite this enthusiasm, the path to developing models with capabilities comparable to those of ChatGPT or GPT-4 remains elusive. The open-source community has been working diligently to bridge the gap between proprietary LLMs and their open-source counterparts. In the past year, several notable open-source LLMs, such as LLaMA ( Touvron et al., 2023a ; Touvron et al., 2023b ) , Qwen ( Bai et al., 2023a ) , Mistral ( Jiang et al., 2023 ) , and Deepseek ( Bi et al., 2024 ) , have made significant strides. In this paper, we introduce InternLM2, a new Large Language Model that outperforms the previously mentioned models. 

The development of Large Language Models (LLMs) encompasses several main phases: pre-training, Supervised Fine-Tuning (SFT), and Reinforcement Learning from Human Feedback (RLHF) ( Ouyang et al., 2022 ) . Pre-training is chiefly based on leveraging a vast corpus of natural text, amassing trillions of tokens. This phase is aimed at equipping LLMs with a broad repository of knowledge and fundamental skills. The quality of data is considered the most crucial factor during pre-training. However, technical reports on LLMs ( Touvron et al., 2023a ; Touvron et al., 2023b ; Bai et al., 2023a ; Bi et al., 2024 ) in the past have seldom addressed the processing of pre-training data. InternLM2 extensively details how it prepares text, code, and long-context data for pre-training. 

How to effectively extend the context length of LLMs is currently a hot research topic, since many downstream applications, such as Retrieval-Augmented Generation (RAG) ( Gao et al., 2023 ) and agents ( Xi et al., 2023 ) , rely on long contexts.
InternLM2 first employs Group Query Attention (GQA) to enable a smaller memory footprint when inferring long sequences. In the pre-training phase, we initially train InternLM2 with 4k context texts, then transit the training corpus to high-quality 32k texts for further training. Upon completion, through positional encoding extrapolation ( LocalLLaMA, 2023 ) , InternLM2 achieves commendable performance in the “Needle-in-a-Haystack” test within 200k contexts. 

Following long-context pre-training, we utilize supervised fine-tuning (SFT) and reinforcement learning from human feedback (RLHF) to ensure the model adheres well to human instructions and aligns with human values. Notably, we also construct corresponding 32k data during these processes to further improve the long-context processing capability of InternLM2. Besides, we introduce CO nditional O n L ine RLHF (COOL RLHF), which adopts a conditional reward model to reconcile diverse but potentially conflicting preferences and executes Proximal Policy Optimization (PPO) over multiple rounds to mitigate emerging reward hacking in each phase. To elucidate the impact of RLHF within the community, we also release models in their pre- and post-RLHF stages, named InternLM2-Chat-{size}-SFT and InternLM2-Chat-{size}, respectively. 

Our contributions are twofold, highlighting not only the model’s exceptional performance across diverse benchmarks but also our comprehensive approach to its development in different stages. Key points include 

1. 

Open-Sourcing InternLM2 with Exceptional Performance: We have open-sourced models of various sizes, including 1.8B, 7B, and 20B, all of which perform well in both subjective and objective evaluations. Additionally, we have released models from different stages to facilitate community analysis of changes post-SFT and RLHF training. 

2. 

Designed with a 200k Context Window: InternLM2 exhibits impressive long-context performance, nearly perfectly identifying all “needles” in the “Needle-in-a-Haystack” experiment with a 200k context. Furthermore, we provide experience of training long-context LLMs across all stages, including pretraining, SFT, and RLHF. 

3. 

Comprehensive Data Preparation Guidance: We have elaborated on the preparation of data for LLMs, including pre-training data, domain-specific enhancement data, SFT data, and RLHF data. These details will benefit the community in better training of LLMs. 

4. 

Innovative RLHF Training Techniques: We introduced Conditional Online RLHF (COOL RLHF) to harmonize various preferences, significantly enhancing the performance of InternLM2 in various subjective dialogue evaluations. We have also conducted a preliminary analysis and comparison of RLHF’s subjective and objective results to offer the community insights into RLHF. 

2 Infrastructure 

In this part, we introduce the training framework InternEvo which was used during pretraining, SFT and RLHF. 

2.1 InternEvo 

We utilize InternEvo, an efficient and lightweight pretraining framework, for model training. This framework enables us to scale model training across thousands of GPUs. This is achieved through a combination of data, tensor ( Shoeybi et al., 2019 ) , sequence ( Korthikanti et al., 2023 ) , and pipeline ( Huang et al., 2019 ) parallelism. To further enhance GPU memory efficiency, InternEvo incorporates various Zero Redundancy Optimizer (ZeRO) ( Rajbhandari et al., 2020 ) strategies, significantly reducing the memory footprint required for training. In addition, to enhance hardware utilization, we incorporate the FlashAttention technique ( Dao, 2023 ) and mixed-precision training ( Narang et al., 2017 ) with BF16. 

InternEvo demonstrates strong scaling performance when training InternLM across thousands of GPUs. As shown in Figure 1 , when training InternLM-7B on 8 GPUs with a global batch size of 4 million tokens, InternEvo achieves 64% Model FLOPs Utilization (MFU). Scaling up the training to 1024 GPUs, InternEvo maintains an impressive 53% MFU with the same global batch size. This level of scaling performance is particularly challenging to attain, as the batch size remains constant and the computation-to-communication ratio decreases with an increased number of GPUs. In contrast, DeepSpeed ( Rasley et al., 2020 ) only manages to achieve around 36% MFU when training the InternLM-7B on 1024 GPUs using ZeRO-1 ( Rajbhandari et al., 2020 ) and MiCS ( Zhang et al., 2022 ) . 

InternEvo also exhibits strong scaling of sequence length, supporting 256,000 256,000 tokens when training LLMs of varying sizes. For instance, when training the InternLM-7B model with a sequence length of 256,000 256,000 tokens, InternEvo can achieve nearly 88% MFU. In comparison, DeepSpeed-Ulysses and Megatron-LM only reach about 65% MFU. This improvement in training performance is also observed with LLMs of larger sizes, such as models with 30 billion or 70 billion parameters. 

Figure 1: Model FLOPs Utilization (MFU) of training InternLM-7B with InternEvo. We benchmark training performance using a sequence length of 4096 tokens with varying GPU numbers, and benchmark training performance on 128 GPUs with varying sequence lengths. Reducing Communication Overhead 

A trade-off exists between memory utilization and communication cost in distributed LLM training. Initially, the communication cost can be effectively reduced by diminishing the communication scale. This involves limiting communications to a smaller group of GPUs, potentially within the same node, which mitigates the overall communication cost. Building upon this principle, InternEvo addresses communication challenges by implementing a suite of adaptive sharding techniques to achieve strong scaling performance ( Chen et al., 2024b ) . These include Full-Replica, Full-Sharding, and Partial-Sharding, which allow each component of the model states—parameters, gradients, and optimizer states—to independently select the most appropriate sharding approach and device mesh configuration. This flexibility facilitates a more nuanced distribution of model states across the GPU infrastructure. InternEvo also introduces an optimization framework designed to identify the most efficient sharding factors. This aims to minimize communication expenses while adhering to the memory constraints of the GPU. 

Communication-Computation Overlap 

Further reducing communication overhead, InternEvo strategically coordinates communication and computation to optimize overall system performance. When employing parameter sharding, the model’s entire parameters are distributed across multiple GPUs to conserve GPU memory. During each forward and backward pass of every micro-batch in a training step, InternEvo efficiently pre-fetches the complete parameter set for upcoming layers through AllGather, while concurrently computing the current layer. The generated gradients undergo synchronization within the parameter sharding group through ReduceScatter and are subsequently synchronized across parameter sharding groups using AllReduce. These communication processes are skillfully overlapped with the backward computation, maximizing efficiency in the training pipeline. In the case of optimizer state sharding, where the GPU broadcasts updated parameters within the sharding group through Broadcast, InternEvo employs a strategic overlap with the forward computation of the next training step. These innovative overlap approaches effectively balance communication overhead and computation execution time, resulting in a significant enhancement in overall system performance. 

Long-Sequence Training 

One of the primary challenges in long-sequence training is the trade-off between computation speed and communication overhead. InternEvo breaks down GPU memory management into a hierarchical space with four parallel dimensions—data, tensor, sequence, and pipeline—and three sharding dimensions—parameter, gradient, and optimizer state ( Chen et al., 2024a ) . We conduct a thorough analysis of memory and communication costs for each dimension, utilizing an execution simulator to identify and implement the optimal parallelization strategy. The optimal execution plan can be automatically searched based on the training scale, sequence length, model size, and batch size. With this execution plan, InternEvo exhibits the capability to handle long contexts (up to 1 million tokens) during training. InternEvo also implements memory management techniques to reduce GPU memory fragmentation, a common issue in long-sequence training scenarios. It uses a memory pool for unified memory management and introduces a defragmentation technique that proactively consolidates small memory chunks to prevent out-of-memory errors. 

Fault Tolerance 

We have also addressed the challenges of efficiently training LLMs in GPU datacenters, which often face issues such as frequent hardware failures, complex parallelization strategies, and imbalanced resource utilization. To tackle these issues, we conducted an in-depth characterization study of a six-month LLM development workload trace from our GPU datacenter ( Hu et al., 2024 ) . This study identified discrepancies between LLMs and previous deep learning workloads and explored resource utilization patterns and job failure impacts. Based on our analysis, we introduced two system efforts: a fault-tolerant pretraining system that enhances fault tolerance through LLM-involved failure diagnosis and automatic recovery, and a decoupled scheduling system for evaluation tasks that provides timely model performance feedback.
In our implementation, we have incorporated an asynchronous saving mechanism that regularly archives model weights and optimizer states to distributed file and object storage at predefined intervals. Throughout the training process, each GPU first saves its model states in local storage and subsequently asynchronously uploads this data to the remote distributed storage system. This dual-step process ensures that, in the event of occasional hardware or network failures automatically detected by our system, only a minimal amount of training progress is lost. To optimize storage space, we systematically transfer these temporary model checkpoints from hot storage to cost-effective cold storage. Furthermore, our system is designed to seamlessly resume model training even when the parallelization configuration is altered, providing flexibility and continuity in the training pipeline. 

Interactive Training 

The efficiency of InternEvo has also been successfully demonstrated in the Reinforcement Learning from Human Feedback (RLHF) stage, where multiple LLMs are deployed for interactive training. For instance, in the Proximal Policy Optimization (PPO) process, we utilize four equally-sized models and train two of them; InternEvo enables each model to be executed at its optimal configuration. To enhance the coordination of multiple models, we have developed an innovative RLHF framework that builds upon InternEvo and Ray. This framework is characterized by its flexibility and scalability, allowing it to perform effectively at scale. It is capable of integrating with various LLM execution engines and supporting diverse algorithmic designs. For a comprehensive description of the ”Alignment” concept, please refer to Section 4 . 

2.2 Model Structure 

Transformer ( Vaswani et al., 2017 ) has been predominantly used as the backbone for past Large Language Models (LLMs) due to its excellent parallelization capabilities, which fully leverage the power of GPUs ( Brown et al., 2020 ; Chowdhery et al., 2023 ; Zeng et al., 2023 ) . LLaMA ( Touvron et al., 2023a ) builds on Transformer architecture by replacing LayerNorm ( Ba et al., 2016 ) with RMSNorm ( Zhang & Sennrich, 2019 ) and setting the activation function to SwiGLU ( Shazeer, 2020 ) , resulting in improved training efficiency and performance.
Since the unveiling of LLaMA ( Touvron et al., 2023a ) , the community has vigorously engaged in augmenting the ecosystem built around the LLaMA architecture. This includes advancements in high-efficiency inference ( lla, 2023 ) and operator optimization ( Dao, 2023 ) , among others. To ensure our model, InternLM2, aligns seamlessly with this well-established ecosystem, alongside other renowned LLMs, such as Falcon ( Almazrouei et al., 2023 ) , Qwen ( Bai et al., 2023a ) , Baichuan ( Yang et al., 2023 ) , Mistral ( Jiang et al., 2023 ) , we opt to adhere to the structural design principles of LLaMA. In pursuit of enhancing efficiency, we consolidate the W k W_{k} , W q W_{q} , and W v W_{v} matrices, which has resulted in a training acceleration of over 5% during the pre-training phase. Moreover, to better support diverse tensor parallelism (tp) transformations, we have reconfigured the matrix layout. Rather than stacking the W k W_{k} , W q W_{q} , and W v W_{v} matrices in a straightforward manner, we adopt an interleaving approach for each head’s W k W_{k} , W q W_{q} , and W v W_{v} , as depicted in Figure 2 . This design modification facilitates the adjustment of the tensor parallel size through either splitting or concatenating the matrices along their last dimension, thereby enhancing the model’s flexibility in varying distributed computing environments. InternLM2 aims to infer beyond 32K context, therefore, InternLM2 series models all choose Grouped-Query Attention (GQA) ( Ainslie et al., 2023 ) , so that it can infer both in high speed and low GPU memory with very long contexts. 

Figure 2: Different weight matrix layouts cause different complexity when changing the tensor parallelism (TP) size. 
3 Pre-train 

We introduce pre-training data, pre-training settings, and three pre-training phases in this part. 

3.1 Pre-training data 

The pre-training of LLMs is critically shaped by data, which presents a multifaceted challenge. It encompasses handling sensitive data, covering comprehensive knowledge, and balancing efficiency and quality. In this section, we will depict our data processing pipeline for preparing general domain textual data, programming language-related data, and long textual data. 

3.1.1 Text Data 

The text data in our pre-training dataset can be categorized by source into web pages, papers, patents, and books. To transform these sources into a pre-training dataset, we first standardize all data into a specified format, categorize them by type and language, and store them in JSON Lines (jsonl) format. Then, for all data, we apply several processing steps including rule-based filtering, data deduplication, safety filtering, and quality filtering. This results in a rich, safe, and high-quality text dataset. 

Data Source Distribution 

We have statistically analyzed the number of documents, data storage size, and data Bytes proportion in the pre-training dataset according to their sources, as shown in Table 1 . Among them, the Chinese and English data from web pages account for 86.46% of the total, making it the primary source. Although the data volume from other sources is relatively small, such as books and technical literature(abbreviated as techlit), the average document length is longer and the content quality is relatively higher, making them equally important. 

Source Docs (M rows) Bytes (GB) Bytes-percent 

en-books 0.50 220.14 1.63% 

en-techlit 59.27 576.48 4.27% 

en-webpages 3614.07 9129.39 67.51% 

zh-books 0.71 366.82 2.71% 

zh-techlit 89.59 668.19 4.94% 

zh-webpages 928.94 2562.86 18.95% 
Table 1: Summary of the pre-train data from different sources Data Process Pipeline 

The data processing pipeline used in this work is shown in Figure 3 . The entire data processing pipeline first standardizes data from different sources to obtain Format data . Then, heuristic statistical rules are used for data filtering to get Clean data . Next, the Locality-Sensitive Hashing (LSH) method is used for data deduplication to obtain Dedup data . We then apply a composite safety strategy to filter the data, resulting in Safe data . We have adopted different quality filtering strategies for data from various sources, ultimately obtaining High-quality pre-training data . 

Figure 3: Data Process Pipeline Data Formatting 

We will detail the data processing pipeline using web page data as an example. Our web page data mainly comes from Common Crawl 1 1 1 https://commoncrawl.org/ . Firstly, we need to decompress the original Warc format files and use Trafilatura ( Barbaresi, 2021 ) for HTML parsing and main text extraction. Then, we use the pycld2 2 2 2 https://pypi.org/project/pycld2/ library for language detection and classification of the main text. Finally, we assign a unique identifier to the data and store it in jsonl (JSON lines) format and obtained Format data . 

Rule-based Stage 

Web page data randomly extracted from the internet often contains a large amount of low-quality data, such as parsing errors, formatting errors, and non-natural language text. A common practice is to design rule-based regularization and filtering methods to modify and filter the data, as seen in Gopher ( Rae et al., 2021 ) , C4 ( Dodge et al., 2021 ) , and RefinedWeb ( Penedo et al., 2023 ) . Based on our observations of the data, we have designed a series of heuristic filtering rules that focus on anomalies in separation and line breaks, frequency of abnormal characters, and distribution of punctuation marks. By applying these filters, we obtained Clean data . 

Deduplication 

A large amount of duplicate texts exist on the Internet, which can negatively impact model training. Therefore, we employed a method based on Locality-Sensitive Hashing (LSH) to perform fuzzy deduplication on the data. More specifically, we used the MinHash method ( Broder, 1997 ) , establishing signatures with 128 hash functions on the 5-gram of the documents, and using 0.7 as the threshold for deduplication. We aimed to retain the most recent data, that is, prioritizing data with larger CC dumps numbers. We obtained the Dedup data after LSH deduplication. 

Safety Filtering 

The internet is rife with toxic and pornographic content, the use of which for model training can negatively impact performance and increase the likelihood of unsafe content generation. Therefore, we employed a comprehensive safety strategy combining “domain blocking”, “word blocking”, “pornography classifier”, and “toxicity classifier” to filter the data. Specifically, we constructed a block domain list comprising approximately 13M unsafe domains and a block word list containing 36,289 unsafe words for preliminary data filtering. Given that word blocking might inadvertently exclude a large amount of data, we opted for a cautious approach in compiling the block word list. 

To further improve the detection rate of unsafe content, we fine-tuned the BERT model using the “Toxic Comment Classification Challenge” dataset from Kaggle, resulting in a toxicity classifier. We sampled some data from Dedup data and annotated it using the Perspective API 3 3 3 https://perspectiveapi.com/ to create a pornography classification dataset. We then fine-tuned the BERT model with this dataset, yielding a pornography classifier. Finally, we used these two classifiers for secondary filtering of the data, filtering out data with scores below the threshold, resulting in Safe data . 

Quality Filtering 

Compared to sources such as books, papers, and patents, internet-sourced data contains a significant amount of low-quality content. Based on our observations, the primary causes for this low-quality content are twofold: 1. The internet is rife with marketing advertisements, which tend to be repetitive and carry little information. 2. Many web pages consist of lists of article abstracts or product descriptions, resulting in extracted text that is difficult to read and lacks logical coherence. 

To filter out these low-quality contents, we first organized manual data annotation. For the advertisements classification task, annotators were asked to identify whether a piece of data contains advertising content (both overall and partial advertisements are marked as low quality). For the fluency classification task, annotators were asked to rate the data on four dimensions: consistency, noise, information content, and grammar, resulting in a comprehensive fluency score. We then fine-tuned the BERT model using the manually annotated data, obtaining an advertisements classifier and a fluency classifier. Finally, we used these two classifiers for secondary filtering of the data, filtering out data with scores below the threshold, resulting in High-quality pre-trai
