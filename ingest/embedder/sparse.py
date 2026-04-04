"""
sparse.py — BM25 sparse vector computation for hybrid retrieval.

Builds a term vocabulary from a corpus of text chunks, computes BM25 sparse
vectors per chunk (for Qdrant sparse vector upsert), and encodes query text
to the same sparse vector space at query time.

Vocab is persisted as JSON: data/vocab/<tenant_id>.json
"""

import json
import math
import os
import re
import string
from typing import Dict, List, Optional, Tuple

try:
    import bm25s
    _BM25S_AVAILABLE = True
except ImportError:
    _BM25S_AVAILABLE = False

STOPWORDS = frozenset([
    "the", "a", "an", "is", "are", "was", "were", "be", "been", "being",
    "have", "has", "had", "do", "does", "did", "will", "would", "could",
    "should", "may", "might", "shall", "can", "of", "in", "to", "for",
    "with", "on", "at", "from", "by", "about", "as", "into", "through",
    "and", "or", "but", "not", "this", "that", "which", "who", "whom",
    "under", "section", "act", "any", "every", "such", "where", "when",
])


def tokenize(text: str) -> List[str]:
    """Lowercase, strip punctuation, remove stopwords."""
    text = text.lower()
    text = text.translate(str.maketrans("", "", string.punctuation))
    return [t for t in text.split() if t and t not in STOPWORDS and len(t) > 1]


class SparseVocab:
    """Vocabulary mapping terms to integer indices."""

    def __init__(self) -> None:
        self.term_to_id: Dict[str, int] = {}
        self.id_to_term: Dict[int, str] = {}
        self.df: Dict[int, int] = {}    # document frequency per term id
        self.num_docs: int = 0

    def build(self, corpus: List[List[str]]) -> None:
        """Build vocab + document frequencies from tokenized corpus."""
        self.num_docs = len(corpus)
        for tokens in corpus:
            seen: set = set()
            for t in tokens:
                if t not in self.term_to_id:
                    idx = len(self.term_to_id)
                    self.term_to_id[t] = idx
                    self.id_to_term[idx] = t
                term_id = self.term_to_id[t]
                if term_id not in seen:
                    self.df[term_id] = self.df.get(term_id, 0) + 1
                    seen.add(term_id)

    def encode(self, tokens: List[str]) -> Tuple[List[int], List[float]]:
        """
        Encode a list of tokens as a sparse TF-IDF vector.
        Returns (indices, values) for Qdrant SparseVector.
        Unknown tokens are silently ignored.
        """
        tf: Dict[int, int] = {}
        for t in tokens:
            if t in self.term_to_id:
                term_id = self.term_to_id[t]
                tf[term_id] = tf.get(term_id, 0) + 1

        if not tf:
            return [], []

        indices: List[int] = []
        values: List[float] = []
        n_docs = max(self.num_docs, 1)

        for term_id, freq in tf.items():
            df = self.df.get(term_id, 1)
            idf = math.log((n_docs + 1) / (df + 1)) + 1.0
            indices.append(term_id)
            values.append(float(freq) * idf)

        return indices, values

    def save(self, path: str) -> None:
        os.makedirs(os.path.dirname(path), exist_ok=True)
        data = {
            "term_to_id": self.term_to_id,
            "df": {str(k): v for k, v in self.df.items()},
            "num_docs": self.num_docs,
        }
        with open(path, "w") as f:
            json.dump(data, f, separators=(",", ":"))

    @classmethod
    def load(cls, path: str) -> "SparseVocab":
        with open(path) as f:
            data = json.load(f)
        vocab = cls()
        vocab.term_to_id = data["term_to_id"]
        vocab.id_to_term = {v: k for k, v in vocab.term_to_id.items()}
        vocab.df = {int(k): v for k, v in data["df"].items()}
        vocab.num_docs = data["num_docs"]
        return vocab


class SparseEncoder:
    """
    Builds a SparseVocab from a corpus of text chunks and encodes them.

    Usage (ingest time):
        encoder = SparseEncoder(tenant_id="public")
        encoder.fit(chunks)          # builds vocab
        vectors = encoder.encode_corpus(chunks)  # [(indices, values), ...]
        encoder.save_vocab("data/vocab/public.json")

    Usage (query time):
        encoder = SparseEncoder.from_vocab("data/vocab/public.json")
        indices, values = encoder.encode_query("murder punishment BNS")
    """

    def __init__(self, tenant_id: str = "public") -> None:
        self.tenant_id = tenant_id
        self.vocab: Optional[SparseVocab] = None

    def fit(self, texts: List[str]) -> None:
        """Build vocabulary from raw texts."""
        tokenized = [tokenize(t) for t in texts]
        self.vocab = SparseVocab()
        self.vocab.build(tokenized)

    def encode_corpus(self, texts: List[str]) -> List[Tuple[List[int], List[float]]]:
        """Encode all texts. Must call fit() first."""
        if self.vocab is None:
            raise RuntimeError("Call fit() before encode_corpus()")
        result = []
        for text in texts:
            tokens = tokenize(text)
            indices, values = self.vocab.encode(tokens)
            result.append((indices, values))
        return result

    def encode_query(self, query: str) -> Tuple[List[int], List[float]]:
        """Encode a single query string."""
        if self.vocab is None:
            raise RuntimeError("Vocab not loaded. Call fit() or from_vocab().")
        tokens = tokenize(query)
        return self.vocab.encode(tokens)

    def save_vocab(self, path: str) -> None:
        if self.vocab is None:
            raise RuntimeError("No vocab to save. Call fit() first.")
        self.vocab.save(path)

    @classmethod
    def from_vocab(cls, path: str, tenant_id: str = "public") -> "SparseEncoder":
        enc = cls(tenant_id=tenant_id)
        enc.vocab = SparseVocab.load(path)
        return enc

    @property
    def vocab_size(self) -> int:
        return len(self.vocab.term_to_id) if self.vocab else 0
