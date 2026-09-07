"""Behavioural tests.

Assert on the outcomes that matter, never on exact probabilities -- those
move every time the seed data changes and a brittle test gets deleted rather
than fixed.

THRESHOLD mirrors SAFETY_THRESHOLD in .env. If you retune one, retune both.
"""
import pytest

from train import build_pipeline, load

THRESHOLD = 0.55


@pytest.fixture(scope="module")
def model():
    X, y = load()
    pipe = build_pipeline()
    pipe.fit(X, y)
    return pipe


def p_crisis(model, text: str) -> float:
    return model.predict_proba([text])[0][1]


@pytest.mark.parametrize("text", [
    "I want to end my life",
    "I do not want to wake up tomorrow",
    "everyone would be better off if I was gone",
    "I have been hurting myself again",
    "I am tired of being alive",
])
def test_flags_crisis_language(model, text):
    assert p_crisis(model, text) > THRESHOLD


# Hard negatives: burnout talk that borrows violent idiom. These are the
# cases that decide whether the tool is usable at all.
@pytest.mark.parametrize("text", [
    "I am completely exhausted by this job",
    "deadlines are murder this month",
    "I killed that presentation today",
    "this workload is killing my motivation",
    "my manager is driving me insane with these targets",
    "I am dead tired after another twelve hour shift",
])
def test_does_not_flag_ordinary_burnout_talk(model, text):
    assert p_crisis(model, text) < THRESHOLD


def test_recall_is_prioritised_over_precision(model):
    """A documented value judgement, encoded as a test.

    A false positive shows someone a helpline card. A false negative routes a
    person in crisis to a chatbot. If a future change flips this trade-off,
    this test should fail and make someone argue for it out loud.
    """
    X, y = load()
    preds = [p_crisis(model, t) > THRESHOLD for t in X]
    tp = sum(1 for p, label in zip(preds, y) if p and label == 1)
    fn = sum(1 for p, label in zip(preds, y) if not p and label == 1)
    recall = tp / (tp + fn)
    assert recall >= 0.95, f"recall dropped to {recall:.2f} on the seed set"
