"""Tests for the statute chunker."""

from ingest.chunker.statute import (
    Chunk,
    parse_constitution,
    chunk_by_pages,
    content_hash,
    _build_parent_summary,
)


SAMPLE_CONSTITUTION_TEXT = """
PART I

1. Name and territory of the Union.—
(1) India, that is Bharat, shall be a Union of States.
(2) The States and the territories thereof shall be as specified in the First Schedule.

2. Admission or establishment of new States.—
Parliament may by law admit into the Union, or establish, new States on such terms and conditions as it thinks fit.

PART II

3. Citizenship at the commencement of the Constitution.—
At the commencement of this Constitution, every person who has his domicile in the territory of India and who was born in the territory of India shall be a citizen of India.

PART III

CHAPTER I — GENERAL

14. Equality before law.—
The State shall not deny to any person equality before the law or the equal protection of the laws within the territory of India.

21. Protection of life and personal liberty.—
No person shall be deprived of his life or personal liberty except according to procedure established by law.
"""


class TestParseConstitution:
    def test_produces_chunks(self):
        chunks = parse_constitution(SAMPLE_CONSTITUTION_TEXT)
        assert len(chunks) > 0

    def test_preserves_article_numbers(self):
        chunks = parse_constitution(SAMPLE_CONSTITUTION_TEXT)
        article_numbers = [c.metadata.get("article_number") for c in chunks]
        assert "1" in article_numbers
        assert "2" in article_numbers
        assert "14" in article_numbers
        assert "21" in article_numbers

    def test_preserves_part_hierarchy(self):
        chunks = parse_constitution(SAMPLE_CONSTITUTION_TEXT)
        for c in chunks:
            if c.metadata.get("article_number") == "1":
                assert "Part I" in c.metadata.get("part", "")
            if c.metadata.get("article_number") == "3":
                assert "Part II" in c.metadata.get("part", "")
            if c.metadata.get("article_number") == "14":
                assert "Part III" in c.metadata.get("part", "")

    def test_chapter_tracking(self):
        chunks = parse_constitution(SAMPLE_CONSTITUTION_TEXT)
        for c in chunks:
            if c.metadata.get("article_number") == "14":
                chapter = c.metadata.get("chapter", "")
                assert "Chapter" in chapter, f"Expected chapter metadata, got: {chapter}"

    def test_sac_parent_summary(self):
        chunks = parse_constitution(
            SAMPLE_CONSTITUTION_TEXT,
            act_name="Constitution of India",
        )
        for c in chunks:
            assert "[PARENT SUMMARY]" in c.text, "Expected SAC parent summary prefix"
            assert "[CHUNK CONTENT]" in c.text, "Expected chunk content marker"
            assert "Act: Constitution of India" in c.text

    def test_content_hash_in_metadata(self):
        chunks = parse_constitution(SAMPLE_CONSTITUTION_TEXT)
        for c in chunks:
            assert "content_hash" in c.metadata
            assert len(c.metadata["content_hash"]) == 64  # SHA-256 hex

    def test_tenant_id_default_public(self):
        chunks = parse_constitution(SAMPLE_CONSTITUTION_TEXT)
        for c in chunks:
            assert c.metadata.get("tenant_id") == "public"

    def test_ipc_bns_mapping_injection(self):
        ipc_map = {"21": "IPC §19 (Right to life)"}
        chunks = parse_constitution(SAMPLE_CONSTITUTION_TEXT, ipc_bns_map=ipc_map)
        for c in chunks:
            if c.metadata.get("article_number") == "21":
                assert "ipc_equivalent" in c.metadata


class TestChunkByPages:
    def test_produces_chunks(self):
        text = "word " * 1000
        chunks = chunk_by_pages(text, words_per_chunk=100)
        assert len(chunks) == 10

    def test_content_hash(self):
        text = "word " * 100
        chunks = chunk_by_pages(text, words_per_chunk=100)
        for c in chunks:
            assert "content_hash" in c.metadata


class TestContentHash:
    def test_deterministic(self):
        assert content_hash("hello") == content_hash("hello")

    def test_different_inputs(self):
        assert content_hash("hello") != content_hash("world")


class TestBuildParentSummary:
    def test_includes_all_fields(self):
        summary = _build_parent_summary(
            "BNS 2023", "Part I", "Chapter III", "103", "Punishment for murder"
        )
        assert "BNS 2023" in summary
        assert "Part I" in summary
        assert "Chapter III" in summary
        assert "Section 103" in summary
