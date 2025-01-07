import functools
import time
from collections.abc import Callable
from typing import ParamSpec, TypeVar

P = ParamSpec("P")
R = TypeVar("R")


def async_timer(logger=None) -> Callable[..., Callable[P, R]]:
    """Decorator factory to measure and log the execution time of async functions."""

    def decorator(func: Callable[P, R]) -> Callable[P, R]:
        @functools.wraps(func)
        async def wrapper(*args, **kwargs):
            start_time = time.time()
            try:
                return await func(*args, **kwargs)
            finally:
                elapsed_time = time.time() - start_time
                if logger:
                    _msg = f"{func.__module__}.{func.__name__} took {elapsed_time:.2f} seconds"
                    logger.debug(_msg)

        return wrapper

    return decorator
