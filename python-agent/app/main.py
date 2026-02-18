"""Python agent entrypoint."""

import asyncio
import logging
import signal
import sys

import structlog

from app.config import get_settings
from app.server import AgentServer


def setup_logging(level: str) -> None:
    structlog.configure(
        processors=[
            structlog.stdlib.filter_by_level,
            structlog.stdlib.add_logger_name,
            structlog.stdlib.add_log_level,
            structlog.processors.TimeStamper(fmt="iso"),
            structlog.processors.format_exc_info,
            (
                structlog.dev.ConsoleRenderer()
                if sys.stdout.isatty()
                else structlog.processors.JSONRenderer()
            ),
        ],
        context_class=dict,
        logger_factory=structlog.stdlib.LoggerFactory(),
        wrapper_class=structlog.stdlib.BoundLogger,
        cache_logger_on_first_use=True,
    )

    logging.basicConfig(
        format="%(message)s",
        stream=sys.stdout,
        level=getattr(logging, level.upper(), logging.INFO),
    )


async def main() -> None:
    settings = get_settings()
    setup_logging(settings.log_level)

    logger = structlog.get_logger()
    logger.info(
        "Starting Python Agent Service",
        version=settings.service_version,
        port=settings.grpc_port,
    )

    server = AgentServer(settings)

    loop = asyncio.get_running_loop()
    for sig in (signal.SIGTERM, signal.SIGINT):
        loop.add_signal_handler(sig, lambda s=sig: asyncio.create_task(server.stop()))  # noqa: ARG005

    try:
        await server.start()
        await server.wait_for_termination()
    finally:
        await server.stop()
        logger.info("Python Agent Service stopped")


if __name__ == "__main__":
    asyncio.run(main())
