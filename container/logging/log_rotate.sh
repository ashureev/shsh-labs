#!/bin/bash
# SHSH Log Rotation Script
# Rotates session.log when it exceeds the size limit
# Can be called manually or by cron

SHSH_LOG_DIR="${SHSH_LOG_DIR:-$HOME/.shsh}"
SHSH_LOG_FILE="$SHSH_LOG_DIR/session.log"
SHSH_MAX_LOG_SIZE="${SHSH_MAX_LOG_SIZE:-10485760}"  # 10MB default

rotate_log() {
    if [[ ! -f "$SHSH_LOG_FILE" ]]; then
        echo "No log file to rotate."
        return 0
    fi
    
    local size=$(stat -c%s "$SHSH_LOG_FILE" 2>/dev/null || echo 0)
    
    if [[ $size -gt $SHSH_MAX_LOG_SIZE ]]; then
        echo "Rotating log (size: $size bytes, limit: $SHSH_MAX_LOG_SIZE bytes)"
        
        # Rotate: .2 -> delete, .1 -> .2, current -> .1
        [[ -f "${SHSH_LOG_FILE}.2" ]] && rm -f "${SHSH_LOG_FILE}.2"
        [[ -f "${SHSH_LOG_FILE}.1" ]] && mv "${SHSH_LOG_FILE}.1" "${SHSH_LOG_FILE}.2"
        mv "$SHSH_LOG_FILE" "${SHSH_LOG_FILE}.1"
        
        echo "Log rotated successfully."
    else
        echo "Log size ($size bytes) below limit ($SHSH_MAX_LOG_SIZE bytes). No rotation needed."
    fi
}

# Show log statistics
show_stats() {
    echo "=== SHSH Session Log Statistics ==="
    echo "Log directory: $SHSH_LOG_DIR"
    echo ""
    
    for log in "$SHSH_LOG_FILE" "${SHSH_LOG_FILE}.1" "${SHSH_LOG_FILE}.2"; do
        if [[ -f "$log" ]]; then
            local size=$(stat -c%s "$log" 2>/dev/null || echo 0)
            local lines=$(wc -l < "$log" 2>/dev/null || echo 0)
            local human_size=$(numfmt --to=iec $size 2>/dev/null || echo "${size}B")
            echo "$(basename $log): $human_size ($lines commands)"
        fi
    done
}

# Parse arguments
case "${1:-rotate}" in
    rotate)
        rotate_log
        ;;
    stats)
        show_stats
        ;;
    force)
        # Force rotation regardless of size
        if [[ -f "$SHSH_LOG_FILE" ]]; then
            [[ -f "${SHSH_LOG_FILE}.2" ]] && rm -f "${SHSH_LOG_FILE}.2"
            [[ -f "${SHSH_LOG_FILE}.1" ]] && mv "${SHSH_LOG_FILE}.1" "${SHSH_LOG_FILE}.2"
            mv "$SHSH_LOG_FILE" "${SHSH_LOG_FILE}.1"
            echo "Log force-rotated."
        else
            echo "No log file to rotate."
        fi
        ;;
    clean)
        # Remove all rotated logs, keep current
        rm -f "${SHSH_LOG_FILE}.1" "${SHSH_LOG_FILE}.2"
        echo "Rotated logs cleaned."
        ;;
    *)
        echo "Usage: $0 [rotate|stats|force|clean]"
        echo ""
        echo "Commands:"
        echo "  rotate  - Rotate log if over size limit (default)"
        echo "  stats   - Show log statistics"
        echo "  force   - Force rotation regardless of size"
        echo "  clean   - Remove rotated logs, keep current"
        exit 1
        ;;
esac
