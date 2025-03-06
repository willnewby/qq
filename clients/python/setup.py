from setuptools import setup, find_packages

setup(
    name="qq-client",
    version="0.1.0",
    description="Python client for qq job queue",
    author="QQ Team",
    packages=find_packages(),
    install_requires=[
        "psycopg>=3.1.0",
    ],
    python_requires=">=3.7",
    classifiers=[
        "Development Status :: 3 - Alpha",
        "Intended Audience :: Developers",
        "License :: OSI Approved :: MIT License",
        "Programming Language :: Python :: 3",
        "Programming Language :: Python :: 3.7",
        "Programming Language :: Python :: 3.8",
        "Programming Language :: Python :: 3.9",
        "Programming Language :: Python :: 3.10",
    ],
)