"""Train the crisis-detection classifier.

Deliberately a simple, inspectable model: TF-IDF over word and character
n-grams into logistic regression. Three reasons, in order of importance:

1. You can print the learned coefficients and see exactly which tokens drive a
   flag. For a safety component that is worth far more than a couple of points
   of F1. If you cannot explain why it fired, you cannot defend it.
2. It trains in under a second on a laptop, so iterating on the *data* -- which
   is where the real gains are -- is free.
3. Character n-grams catch misspellings, spacing tricks and typos that word
   features miss entirely ("k1ll", "s u i c i d e", "killl").

Recall matters more than precision here. A false positive shows someone a
helpline card. A false negative routes a person in crisis to a chatbot.
class_weight="balanced" pushes the model toward recall on the rare class.
"""

import csv
import pathlib

import joblib
from sklearn.feature_extraction.text import TfidfVectorizer
from sklearn.linear_model import LogisticRegression
from sklearn.metrics import classification_report
from sklearn.model_selection import train_test_split
from sklearn.pipeline import FeatureUnion, Pipeline

HERE = pathlib.Path(__file__).parent
MODEL_PATH = HERE / "models" / "crisis_clf.joblib"


def load(path=HERE / "data" / "seed.csv"):
    rows = list(csv.DictReader(path.open()))
    return [r["text"] for r in rows], [int(r["label"]) for r in rows]


def build_pipeline() -> Pipeline:
    # A Pipeline, not a vectorizer plus a model held separately: the fitted
    # vocabulary travels with the weights in one .joblib file. Serving a model
    # whose vectorizer was fitted on different data is a classic, silent,
    # extremely hard-to-debug production failure.
    features = FeatureUnion(
        [
            ("word", TfidfVectorizer(ngram_range=(1, 2), sublinear_tf=True, min_df=1)),
            ("char", TfidfVectorizer(analyzer="char_wb", ngram_range=(3, 5), min_df=1)),
        ]
    )
    return Pipeline(
        [
            ("features", features),
            ("clf", LogisticRegression(max_iter=1000, class_weight="balanced", C=1.0)),
        ]
    )


def explain(pipe: Pipeline, n: int = 12) -> None:
    """Print the tokens pulling hardest in each direction.

    Run this every time there is change in the data. It is the whole reason this model
    is a linear one, and it will show a bad label faster than any metric.
    """
    names = pipe.named_steps["features"].get_feature_names_out()
    coefs = pipe.named_steps["clf"].coef_[0]
    order = coefs.argsort()
    print("\ntop 'crisis' features:")
    for i in reversed(order[-n:]):
        print(f"  {coefs[i]:+.3f}  {names[i]!r}")
    print("\ntop 'ok' features:")
    for i in order[:n]:
        print(f"  {coefs[i]:+.3f}  {names[i]!r}")


def main():
    X, y = load()
    # stratify=y keeps the class balance in both splits. Without it, a small
    # dataset can hand you a test set with almost no positives and a recall
    # figure that means nothing.
    X_tr, X_te, y_tr, y_te = train_test_split(
        X, y, test_size=0.25, random_state=42, stratify=y
    )

    pipe = build_pipeline()
    pipe.fit(X_tr, y_tr)

    print(
        classification_report(
            y_te, pipe.predict(X_te), target_names=["ok", "crisis"], zero_division=0
        )
    )
    explain(pipe)

    # Refit on everything before shipping: the split existed to measure, and
    # with a dataset this small you want every labelled row in the model.
    pipe.fit(X, y)
    MODEL_PATH.parent.mkdir(exist_ok=True)
    joblib.dump(pipe, MODEL_PATH)
    print(f"\nsaved -> {MODEL_PATH}")


if __name__ == "__main__":
    main()
