from worker.quality_gate.gate import check_content


def test_quality_gate_rejects_short():
    r = check_content("hi", min_length=20)
    assert not r.passed
    assert r.reason == "content_too_short"


def test_quality_gate_accepts_normal():
    text = "GrayDraft Worker 负责文档解析、语义分块与质量门禁，确保入库内容可靠。"
    r = check_content(text, min_length=20)
    assert r.passed


def test_scientific_english_is_not_rejected_for_language():
    text = 'The observatory measured a gravitational wave signal with a frequency of 150 Hz and a signal-to-noise ratio of 32.4. The reported uncertainty depends on the observational assumptions.'
    result = check_content(text, tags=['smart-import'])
    assert result.passed, result.reason


def test_long_english_is_not_character_entropy_noise():
    text = 'The observatory measures gravitational radiation from compact objects. Scientists compare measured waveforms with numerical simulations and report distances, masses, confidence intervals, calibration methods, independent observations, astrophysical implications and experimental limitations. '
    # Independent variable clauses give vocabulary diversity without relying on Chinese.
    text = ' '.join(text.replace('observatory', f'instrument{i}') for i in range(5))
    assert check_content(text, tags=['smart-import']).passed
    assert not check_content('buy cheap products now ' * 1000, tags=['smart-import']).passed
