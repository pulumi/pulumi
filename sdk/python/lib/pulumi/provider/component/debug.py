from datetime import datetime


def log(msg: str) -> None:
    current = datetime.now()
    with open("./components.log", "a") as f:
        print(f"[{current.strftime('%m/%d/%Y %H:%M')}] {msg}", file=f)
