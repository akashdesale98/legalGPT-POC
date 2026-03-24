import re
import fitz
from dataclasses import dataclass, asdict
from typing import List


@dataclass
class LegalSection:
    part: str | None
    part_title: str | None
    chapter: str | None
    chapter_title: str | None
    article: int
    title: str
    clauses: List[str]
    text: str


def extract_pdf_text(path):
    doc = fitz.open(path)
    text = ""
    for page in doc:
        text += page.get_text()
    return text


PART_PATTERN = re.compile(r"PART\s+([IVXLC]+)\s*\n([A-Z\s]+)")
CHAPTER_PATTERN = re.compile(r"CHAPTER\s+([IVXLC]+).*?\n([A-Z\s]+)")
ARTICLE_PATTERN = re.compile(r"\n(\d+)\.\s+([^\n]+)")
CLAUSE_PATTERN = re.compile(r"\(\d+\)|\([a-z]\)")


def parse_constitution(text):

    parts = list(PART_PATTERN.finditer(text))
    articles = list(ARTICLE_PATTERN.finditer(text))

    results = []

    current_part = None
    current_part_title = None
    current_chapter = None
    current_chapter_title = None

    part_index = 0
    chapter_index = 0

    chapters = list(CHAPTER_PATTERN.finditer(text))

    for i, article_match in enumerate(articles):

        article_num = int(article_match.group(1))
        title = article_match.group(2).strip()

        start = article_match.end()

        if i + 1 < len(articles):
            end = articles[i + 1].start()
        else:
            end = len(text)

        body = text[start:end].strip()

        while part_index < len(parts) and parts[part_index].start() < article_match.start():
            current_part = parts[part_index].group(1)
            current_part_title = parts[part_index].group(2).strip()
            part_index += 1

        while chapter_index < len(chapters) and chapters[chapter_index].start() < article_match.start():
            current_chapter = chapters[chapter_index].group(1)
            current_chapter_title = chapters[chapter_index].group(2).strip()
            chapter_index += 1

        clauses = [c.strip() for c in re.split(CLAUSE_PATTERN, body) if c.strip()]

        section = LegalSection(
            part=current_part,
            part_title=current_part_title,
            chapter=current_chapter,
            chapter_title=current_chapter_title,
            article=article_num,
            title=title,
            clauses=clauses,
            text=body
        )

        results.append(asdict(section))

    return results

text = extract_pdf_text("constitution.pdf")

sections = parse_constitution(text)

print(sections[0])