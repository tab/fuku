"""Speeds up the waits of a raw take: python3 cut.py <marks> <cut> <raw.mp4> <out.mp4>

The cut file holds `offset <seconds>`, the video time of the first mark,
`speed <from> <to> <factor>` lines and an optional `poster <bound>`, the poster frame (default: the last frame). A bound is `begin`, `end` or a mark name with an optional +/- seconds.
Every gap between speed lines plays at normal speed.
"""

import re
import subprocess
import sys


def parse_marks(text):
    """Returns the wall time of each mark"""
    return {name: float(value) for name, value in (line.split() for line in text.splitlines() if line.strip())}


def parse_cut(text):
    """Returns the offset, the speed lines and the poster bound of a cut file"""
    offset, speeds, poster = 0.0, [], "end-0.5"
    for line in text.splitlines():
        words = line.split()
        if not words or words[0].startswith("#"):
            continue
        if words[0] == "offset":
            offset = float(words[1])
        if words[0] == "speed":
            speeds.append((words[1], words[2], float(words[3])))
        if words[0] == "poster":
            poster = words[1]
    return offset, speeds, poster


def resolve(bound, marks, offset, duration):
    """Returns the video time of a bound"""
    match = re.fullmatch(r"([A-Za-z]\w*?)([+-][\d.]+)?", bound)
    name, delta = match.group(1), float(match.group(2) or 0)
    if name == "begin":
        return delta
    if name == "end":
        return duration + delta
    first = min(marks.values())
    return marks[name] - first + offset + delta


def segments(marks, offset, speeds, duration):
    """Returns (start, stop, factor) segments covering the whole take"""
    fast = sorted((resolve(a, marks, offset, duration), resolve(b, marks, offset, duration), f) for a, b, f in speeds)
    result, cursor = [], 0.0
    for start, stop, factor in fast:
        start, stop = max(start, cursor), min(stop, duration)
        if stop <= start:
            continue
        if start > cursor:
            result.append((cursor, start, 1.0))
        result.append((start, stop, factor))
        cursor = stop
    if cursor < duration:
        result.append((cursor, duration, 1.0))
    return result


def check():
    """Asserts the segment math"""
    marks = {"start": 100.0, "failed": 108.0, "done": 140.0}
    offset, speeds, poster = parse_cut("offset 1.5\nspeed start+1 failed 1.5\nspeed failed+5 done 3\n")
    assert poster == "end-0.5"
    got = segments(marks, offset, speeds, 50.0)
    assert got == [(0.0, 2.5, 1.0), (2.5, 9.5, 1.5), (9.5, 14.5, 1.0), (14.5, 41.5, 3.0), (41.5, 50.0, 1.0)], got
    assert resolve("end-6", marks, offset, 50.0) == 44.0
    assert resolve("done2-0.5", {"done2": 110.0}, 1.0, 50.0) == 0.5


def duration_of(path):
    """Returns the length of a video in seconds"""
    out = subprocess.run(["ffprobe", "-v", "error", "-show_entries", "format=duration", "-of", "csv=p=0", path],
                         check=True, capture_output=True, text=True)
    return float(out.stdout)


def main(marks_path, cut_path, raw, output):
    """Writes the cut video and its poster"""
    with open(marks_path) as marks_file, open(cut_path) as cut_file:
        marks = parse_marks(marks_file.read())
        offset, speeds, poster_bound = parse_cut(cut_file.read())
    raw_duration = duration_of(raw)
    parts = segments(marks, offset, speeds, raw_duration)
    chains = [f"[0:v]trim={a:.2f}:{b:.2f},setpts=(PTS-STARTPTS)/{f}[s{i}]" for i, (a, b, f) in enumerate(parts)]
    joined = "".join(f"[s{i}]" for i in range(len(parts)))
    graph = ";".join(chains) + f";{joined}concat=n={len(parts)}:v=1:a=0,fps=30[out]"
    subprocess.run(["ffmpeg", "-loglevel", "error", "-y", "-i", raw, "-filter_complex", graph, "-map", "[out]",
                    "-c:v", "libx264", "-pix_fmt", "yuv420p", "-crf", "20", "-movflags", "+faststart", output], check=True)
    poster = output.rsplit(".", 1)[0] + ".jpg"
    poster_at = resolve(poster_bound, marks, offset, raw_duration)
    subprocess.run(["ffmpeg", "-loglevel", "error", "-y", "-ss", f"{poster_at:.2f}", "-i", raw, "-frames:v", "1",
                    "-q:v", "3", poster], check=True)
    print(f"{output}: {duration_of(output):.1f}s")


if __name__ == "__main__":
    if sys.argv[1:] == ["--check"]:
        check()
        print("ok")
    else:
        main(*sys.argv[1:])
