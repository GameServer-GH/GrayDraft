from worker.chunkers.semantic_chunker import chunk_text


def test_chunk_text_splits_long_document():
    text = "段落一内容。\n\n" + "段落二" * 200
    chunks = chunk_text(text, chunk_size=100, chunk_overlap=10)
    assert len(chunks) >= 2
    assert all(len(c) <= 250 for c in chunks)
