"""Shared pipeline response models."""

from dataclasses import dataclass, field


@dataclass(slots=True)
class PipelineResponse:
    """Response produced by the decision pipeline."""

    type: str
    content: str = ""
    sidebar: str = ""
    silent: bool = False
    alert: str = ""
    require_confirm: bool = False
    pattern: str = ""
    tools_used: list[str] = field(default_factory=list)
    block: bool = False
