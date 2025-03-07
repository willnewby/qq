FROM alpine:latest

# Create a non-root user to run the application
RUN addgroup -S app && adduser -S -g app app

# Copy the pre-built binary
COPY qq /usr/local/bin/

# Copy documentation
COPY LICENSE README.md /app/

# Set ownership to the non-root user
RUN chown -R app:app /app && chmod +x /usr/local/bin/qq

# Switch to the non-root user
USER app

# Set the default command
ENTRYPOINT ["/usr/local/bin/qq"]
CMD ["--help"]