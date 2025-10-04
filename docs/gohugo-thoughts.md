# Hugo Integration for Prospero - Architecture Decision

## Discussion Summary

The goal was to integrate a Hugo static site with the prospero project, which will be deployed as a Magic Container on bunny.net. The key question was whether to:
1. Separate Hugo site (served via bunny.net CDN) from APIs (served via Magic Container)
2. Combine everything into a single Magic Container deployment

## Options Considered

### Option 1: Separated Architecture
**Hugo site** → bunny.net CDN  
**APIs** → Magic Container

### Option 2: All-in-One Architecture (Static Embed)
**Everything** → Single Magic Container (Hugo site + APIs)

### Option 3: In-Memory Hugo with Nightly Sync (Recommended for Social Posts)
**Embedded base** → In-memory updates → Nightly GitHub sync → Container refresh

## Pros and Cons Analysis

### Separated Architecture

**Pros:**
- ✅ Optimal performance - static files served from CDN edge
- ✅ Cost efficient - CDN bandwidth (~$0.01/GB) vs compute costs
- ✅ Independent scaling - static and dynamic scale separately
- ✅ Better caching - CDN handles cache invalidation automatically
- ✅ Smaller containers - API container stays minimal

**Cons:**
- ❌ More complex deployment - manage two services
- ❌ CORS configuration needed
- ❌ Version synchronization challenges
- ❌ Separate CI/CD pipelines
- ❌ Multiple configuration points

### All-in-One Architecture (Static Embed)

**Pros:**
- ✅ **Simplicity** - one deployment, one container, one config
- ✅ **Atomic updates** - frontend and backend always in sync
- ✅ **No CORS issues** - everything on same origin
- ✅ **Easier local development** - same setup everywhere
- ✅ **Single source of truth** - all env vars in one place
- ✅ **Simpler CI/CD** - one build pipeline

**Cons:**
- ❌ Larger container size (initially thought to be an issue)
- ❌ Serving static files uses compute credits
- ❌ Slightly slower cold starts
- ❌ Must implement own caching headers
- ❌ Rebuild everything for any change

### In-Memory Hugo with Nightly Sync

**Pros:**
- ✅ **Immediate updates** - Social posts appear instantly
- ✅ **No GitHub API rate limits** - Batch sync once daily
- ✅ **Static site benefits** - SEO, performance maintained
- ✅ **Self-healing** - Container restarts get fresh embedded data
- ✅ **Operational simplicity** - Still single container
- ✅ **Eventual consistency** - GitHub remains source of truth
- ✅ **Rollback safety** - Bad posts don't break rebuilds

**Cons:**
- ❌ **Memory overhead** - Hugo binary + runtime files
- ❌ **Complex startup** - Initialize working directories
- ❌ **Temporary inconsistency** - Posts lost on restart until sync
- ❌ **Build complexity** - In-memory Hugo rebuilds

## Key Insights

1. **User geography matters** - If users are geographically concentrated, CDN provides less benefit
2. **Image storage strategy** - Storing images in Bunny Storage keeps container size small (~20-35MB)
3. **Management overhead** - Operational simplicity often outweighs performance optimization
4. **Cost at scale** - For moderate traffic, the compute cost difference is negligible

## Decision: All-in-One Architecture

The decision was made to use the **all-in-one approach** because:
- Simplicity is the top priority
- Users are geographically concentrated (CDN less beneficial)
- Images stored externally keep container small
- Operational simplicity outweighs potential cost savings
- Single deployment is easier to manage and reason about

## Architecture Design

### Container Structure
```
prospero (single binary ~15-25MB)
├── Embedded Hugo site (HTML/CSS/JS ~5MB)
├── API handlers (/api/*)
├── Static file server (/* → embedded files)
└── SSH server (port 2222)

Total container size: ~20-35MB
```

### File Organization
```
prospero/
├── cmd/prospero/          # Main binary entry
├── internal/
│   ├── features/
│   │   ├── hugo/           # NEW: Hugo feature
│   │   │   ├── service.go  # Serve embedded files
│   │   │   ├── embed.go    # go:embed directives
│   │   │   └── static/     # Built Hugo site (no images)
│   │   ├── topten/         # Existing feature
│   │   └── shakespert/     # Existing feature
│   └── app/
│       └── server/
│           └── http.go     # Updated routing
├── hugo-site/              # Hugo source (not embedded)
│   ├── content/
│   ├── layouts/
│   ├── static/            # CSS/JS only (no images)
│   └── config.toml
└── deploy/
    └── magic-container/
        └── Containerfile   # Build Hugo during container build
```

### Request Routing
```go
// Priority routing (specific to general)
/health         → Health check endpoint
/api/topten     → Top Ten API
/api/shakespert → Shakespert API  
/api/*          → Future API endpoints
/*              → Embedded Hugo static files
```

### Build Process
```dockerfile
# Container build stages
FROM hugo:latest AS hugo-builder
COPY hugo-site/ /src/
RUN hugo --minify --baseURL "https://prospero.example.com"

FROM golang:1.21 AS go-builder
COPY . /build/
COPY --from=hugo-builder /src/public/ /build/internal/features/hugo/static/
RUN go build -o prospero ./cmd/prospero

FROM scratch
COPY --from=go-builder /build/prospero /prospero
EXPOSE 8080 2222
ENTRYPOINT ["/prospero", "serve"]
```

### Image Handling Strategy

Images are stored separately in Bunny Storage to keep container small:

1. **Development**: Images served locally from `hugo-site/static/images/`
2. **Production**: Images served from `https://cdn.example.b-cdn.net/images/`
3. **Hugo templates** use environment-aware image URLs:
   ```html
   <img src="{{ .Site.Params.imagesCDN }}/{{ .Params.image }}">
   ```

### Implementation Details

#### 1. Hugo Feature Service
```go
// internal/features/hugo/service.go
package hugo

import (
    "embed"
    "io/fs"
    "net/http"
)

//go:embed static/*
var staticFiles embed.FS

type Service struct {
    files fs.FS
}

func NewService() (*Service, error) {
    files, err := fs.Sub(staticFiles, "static")
    if err != nil {
        return nil, err
    }
    return &Service{files: files}, nil
}

func (s *Service) FileServer() http.Handler {
    return http.FileServer(http.FS(s.files))
}
```

#### 2. Updated HTTP Server
```go
// internal/app/server/http.go additions
hugoService, err := hugo.NewService()
if err != nil {
    return fmt.Errorf("failed to initialize hugo service: %w", err)
}

// API routes with /api prefix
r.Route("/api", func(r chi.Router) {
    r.Get("/topten", handlers.TopTen(toptenService))
    r.Get("/shakespert/*", handlers.Shakespert(shakespertService))
})

// Health check
r.Get("/health", handlers.Health())

// Hugo site for everything else (catch-all)
r.Mount("/", middleware.AddCacheHeaders(hugoService.FileServer()))
```

#### 3. Cache Headers Middleware
```go
func AddCacheHeaders(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        // Cache static assets for 1 year
        if strings.HasSuffix(r.URL.Path, ".css") || 
           strings.HasSuffix(r.URL.Path, ".js") ||
           strings.HasSuffix(r.URL.Path, ".woff2") {
            w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
        } else {
            // HTML gets shorter cache
            w.Header().Set("Cache-Control", "public, max-age=3600")
        }
        next.ServeHTTP(w, r)
    })
}
```

### Development Workflow

1. **Local Development**
   ```bash
   # Terminal 1: Run Hugo in watch mode
   cd hugo-site && hugo server -D

   # Terminal 2: Run prospero APIs
   just run serve --http-port 8081
   ```

2. **Build for Production**
   ```bash
   # Build everything
   just build-hugo
   just build

   # Upload images to Bunny Storage (one-time or on change)
   rclone sync hugo-site/static/images/ bunny:zone/images/
   
   # Build container
   just container-magic
   ```

3. **Deployment**
   ```bash
   # Push to registry
   docker push registry.example.com/prospero:latest
   
   # Deploy to Magic Container (via bunny.net dashboard or API)
   ```

### Environment Configuration

```env
# Production environment variables for Magic Container
HUGO_BASE_URL=https://prospero.example.com
IMAGE_CDN_URL=https://cdn.example.b-cdn.net
TOP10_AGE_PASSPHRASE=xxx
IMAGE_SIGNING_SECRET=xxx
```

### Performance Optimizations

1. **Compression**: Enable gzip/brotli in Chi middleware
2. **Caching**: Aggressive cache headers for versioned assets
3. **Minification**: Hugo's `--minify` flag during build
4. **Image Optimization**: Bunny Storage handles WebP conversion
5. **Cold Start**: Small container (~35MB) ensures fast starts

## Future Considerations

1. **Blue-Green Deployments**: Deploy new version alongside old, switch traffic
2. **Feature Flags**: Control feature rollout without rebuilds
3. **A/B Testing**: Serve different Hugo builds based on headers
4. **Monitoring**: Add metrics for static vs dynamic request ratios
5. **CDN Fallback**: Could add CDN in front of Magic Container later if needed

## Conclusion

The all-in-one architecture provides the best balance of simplicity and functionality for the prospero project. By keeping images external and embedding only the essential Hugo files, we achieve a small container size while maintaining operational simplicity. This approach aligns well with the project's philosophy of being a "magical box" - one simple, self-contained unit that does everything needed.

---

# Social Posts Integration - Complete Implementation Plan

## Overview: TOML-Based Social Posts with In-Memory Hugo

This section details the complete implementation for adding social media-style posts to prospero using Hugo's data files with TOML format, combined with in-memory rebuilds and nightly GitHub synchronization.

### Architecture Flow
```
User POST → Add to memory → Update TOML → Rebuild Hugo → Serve immediately
                ↓
         Nightly sync to GitHub → Trigger container rebuild → Fresh start
```

## Data Structure Design

### Social Posts TOML Schema
```toml
# hugo-site/data/social.toml
title = "Social Posts"
description = "Quick thoughts and updates"

[[posts]]
id = "2024-01-15-1420"
date = "2024-01-15T14:20:00Z"
content = "Just deployed prospero to production! 🚀"
tags = ["deployment", "prospero"]
reply_to = ""  # Optional: ID of post being replied to
media = []     # Optional: Array of media URLs

[[posts]]
id = "2024-01-15-1635"
date = "2024-01-15T16:35:00Z"  
content = "Hugo + Go = ❤️\n\nThe combination just works so well for static sites with dynamic APIs."
tags = ["hugo", "golang", "webdev"]
reply_to = ""
media = ["https://cdn.example.b-cdn.net/images/hugo-go-love.webp"]
```

### Hugo Template Integration
```html
<!-- layouts/partials/social-feed.html -->
{{ with .Site.Data.social }}
<section class="social-feed">
    <h2>{{ .title | default "Recent Updates" }}</h2>
    {{ range sort .posts "date" "desc" }}
    <article class="social-post" data-id="{{ .id }}">
        <header class="post-meta">
            <time datetime="{{ .date }}">
                {{ dateFormat "Jan 2, 2006 at 3:04 PM" .date }}
            </time>
            {{ if .reply_to }}
            <span class="reply-indicator">↳ In reply to</span>
            {{ end }}
        </header>
        
        <div class="post-content">
            {{ .content | markdownify }}
        </div>
        
        {{ with .media }}
        <div class="post-media">
            {{ range . }}
            <img src="{{ . }}" loading="lazy" alt="">
            {{ end }}
        </div>
        {{ end }}
        
        {{ with .tags }}
        <footer class="post-tags">
            {{ range . }}
            <span class="tag">#{{ . }}</span>
            {{ end }}
        </footer>
        {{ end }}
    </article>
    {{ end }}
</section>
{{ end }}
```

## Complete Implementation

### 1. Social Posts Service
```go
// internal/features/social/service.go
package social

import (
    "context"
    "fmt"
    "strings"
    "time"
    "sync"
    "os"
    "path/filepath"
    
    "github.com/BurntSushi/toml"
    "github.com/google/go-github/v57/github"
    "golang.org/x/oauth2"
)

type Service struct {
    // In-memory state
    socialData  *SocialData
    isDirty     bool
    mu          sync.RWMutex
    
    // Hugo integration
    hugoService *hugo.LiveService
    
    // GitHub sync
    githubClient *github.Client
    owner        string
    repo         string
    branch       string
    dataPath     string
    
    // Configuration
    syncHour     int    // Hour to sync (0-23)
    syncEnabled  bool
    
    // Channels for orchestration
    syncTrigger  chan struct{}
    shutdownChan chan struct{}
}

type SocialData struct {
    Title       string `toml:"title"`
    Description string `toml:"description"`
    Posts       []Post `toml:"posts"`
}

type Post struct {
    ID       string    `toml:"id"`
    Date     time.Time `toml:"date"`
    Content  string    `toml:"content"`
    Tags     []string  `toml:"tags,omitempty"`
    ReplyTo  string    `toml:"reply_to,omitempty"`
    Media    []string  `toml:"media,omitempty"`
}

func NewService(hugoSvc *hugo.LiveService, cfg Config) (*Service, error) {
    // Initialize GitHub client
    ts := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: cfg.GitHubToken})
    tc := oauth2.NewClient(context.Background(), ts)
    githubClient := github.NewClient(tc)
    
    s := &Service{
        socialData:   &SocialData{
            Title:       "Social Posts",
            Description: "Quick thoughts and updates",
            Posts:       make([]Post, 0),
        },
        hugoService:  hugoSvc,
        githubClient: githubClient,
        owner:        cfg.GitHubOwner,
        repo:         cfg.GitHubRepo,
        branch:       cfg.GitHubBranch,
        dataPath:     cfg.DataPath, // "hugo-site/data/social.toml"
        syncHour:     cfg.SyncHour,
        syncEnabled:  cfg.SyncEnabled,
        syncTrigger:  make(chan struct{}, 1),
        shutdownChan: make(chan struct{}),
    }
    
    // Load initial data from embedded Hugo site or GitHub
    if err := s.initialize(); err != nil {
        return nil, fmt.Errorf("failed to initialize social service: %w", err)
    }
    
    // Start sync scheduler
    if s.syncEnabled {
        go s.syncScheduler()
    }
    
    return s, nil
}

func (s *Service) CreatePost(content string) (*Post, error) {
    s.mu.Lock()
    defer s.mu.Unlock()
    
    // Create new post
    post := Post{
        ID:      s.generateID(),
        Date:    time.Now().UTC(),
        Content: strings.TrimSpace(content),
        Tags:    s.extractHashtags(content),
    }
    
    // Add to in-memory data
    s.socialData.Posts = append(s.socialData.Posts, post)
    s.isDirty = true
    
    // Update Hugo data and rebuild
    if err := s.updateHugoData(); err != nil {
        // Rollback on failure
        s.socialData.Posts = s.socialData.Posts[:len(s.socialData.Posts)-1]
        s.isDirty = false
        return nil, fmt.Errorf("failed to update Hugo: %w", err)
    }
    
    return &post, nil
}

func (s *Service) updateHugoData() error {
    // Serialize to TOML
    tomlData, err := s.serializeToTOML()
    if err != nil {
        return err
    }
    
    // Write to Hugo working directory
    return s.hugoService.UpdateDataFile("social.toml", tomlData)
}

func (s *Service) syncToGitHub() error {
    ctx := context.Background()
    
    // Get current file to get SHA
    fileContent, _, _, err := s.githubClient.Repositories.GetContents(
        ctx, s.owner, s.repo, s.dataPath,
        &github.RepositoryContentGetOptions{Ref: s.branch},
    )
    
    // Serialize current data
    tomlData, err := s.serializeToTOML()
    if err != nil {
        return fmt.Errorf("failed to serialize TOML: %w", err)
    }
    
    // Update or create file
    message := fmt.Sprintf("Nightly sync: %d social posts", len(s.socialData.Posts))
    
    opts := &github.RepositoryContentFileOptions{
        Message: &message,
        Content: tomlData,
        Branch:  &s.branch,
    }
    
    if fileContent != nil {
        opts.SHA = fileContent.SHA
    }
    
    _, _, err = s.githubClient.Repositories.UpdateFile(
        ctx, s.owner, s.repo, s.dataPath, opts,
    )
    if err != nil {
        return fmt.Errorf("failed to update GitHub file: %w", err)
    }
    
    s.isDirty = false
    return nil
}

type Config struct {
    GitHubToken  string
    GitHubOwner  string
    GitHubRepo   string
    GitHubBranch string
    DataPath     string
    SyncHour     int
    SyncEnabled  bool
}
```

### 2. Enhanced Hugo Live Service
```go
// internal/features/hugo/live.go
package hugo

import (
    "embed"
    "fmt"
    "io/fs"
    "os/exec"
    "path/filepath"
    "sync"
    "time"
    
    "github.com/spf13/afero"
)

//go:embed all:static
var embeddedSite embed.FS

type LiveService struct {
    // File system management
    embeddedFS    embed.FS
    workFS        afero.Fs
    
    // Directories
    workDir       string
    outputDir     string
    
    // Build state
    buildMu       sync.RWMutex
    lastBuild     time.Time
    buildQueue    chan struct{}
    
    // Serving
    currentSite   fs.FS
    servingMu     sync.RWMutex
    
    // Configuration
    hugoPath      string
    baseURL       string
    environment   string
}

func (s *LiveService) UpdateDataFile(filename string, content []byte) error {
    dataPath := filepath.Join(s.workDir, "data", filename)
    
    if err := afero.WriteFile(s.workFS, dataPath, content, 0644); err != nil {
        return err
    }
    
    // Trigger rebuild
    s.triggerBuild()
    return nil
}

func (s *LiveService) buildSite() error {
    args := []string{
        "--source", s.workDir,
        "--destination", s.outputDir,
        "--minify",
        "--environment", s.environment,
    }
    
    if s.baseURL != "" {
        args = append(args, "--baseURL", s.baseURL)
    }
    
    cmd := exec.Command(s.hugoPath, args...)
    if err := cmd.Run(); err != nil {
        return fmt.Errorf("hugo build failed: %w", err)
    }
    
    s.lastBuild = time.Now()
    return nil
}
```

### 3. HTTP Handlers for Social Posts
```go
// internal/web/handlers/social.go
package handlers

import (
    "encoding/json"
    "fmt"
    "net/http"
    "strconv"
    
    "prospero/internal/features/social"
)

type SocialHandler struct {
    service *social.Service
}

func NewSocialHandler(service *social.Service) *SocialHandler {
    return &SocialHandler{service: service}
}

func (h *SocialHandler) CreatePost(w http.ResponseWriter, r *http.Request) {
    if r.Method != http.MethodPost {
        http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
        return
    }
    
    var req struct {
        Content string `json:"content"`
    }
    
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        http.Error(w, "Invalid JSON", http.StatusBadRequest)
        return
    }
    
    if req.Content == "" {
        http.Error(w, "Content cannot be empty", http.StatusBadRequest)
        return
    }
    
    if len(req.Content) > 2000 {
        http.Error(w, "Content too long (max 2000 chars)", http.StatusBadRequest)
        return
    }
    
    post, err := h.service.CreatePost(req.Content)
    if err != nil {
        http.Error(w, fmt.Sprintf("Failed to create post: %v", err), http.StatusInternalServerError)
        return
    }
    
    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(map[string]interface{}{
        "success": true,
        "post":    post,
    })
}

func (h *SocialHandler) GetPosts(w http.ResponseWriter, r *http.Request) {
    limitStr := r.URL.Query().Get("limit")
    limit := 20 // Default limit
    
    if limitStr != "" {
        if l, err := strconv.Atoi(limitStr); err == nil && l > 0 && l <= 100 {
            limit = l
        }
    }
    
    posts, err := h.service.GetPosts(limit)
    if err != nil {
        http.Error(w, fmt.Sprintf("Failed to get posts: %v", err), http.StatusInternalServerError)
        return
    }
    
    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(map[string]interface{}{
        "posts": posts,
    })
}
```

## GitHub Sync Mechanism Deep Dive

### Sync Strategy Overview
The GitHub sync mechanism is designed to be robust, efficient, and handle edge cases gracefully:

1. **Batched Updates**: All posts since last sync are committed in a single operation
2. **Conflict Resolution**: Uses GitHub's SHA-based optimistic locking
3. **Error Recovery**: Retries with exponential backoff
4. **Rate Limiting**: Respects GitHub API limits (5000/hour)

### Detailed Sync Implementation
```go
// internal/features/social/github.go
package social

import (
    "context"
    "fmt"
    "time"
    "math/rand"
)

// Enhanced sync scheduler with error handling and retries
func (s *Service) syncScheduler() {
    ticker := time.NewTicker(time.Hour)
    defer ticker.Stop()
    
    // Add jitter to prevent thundering herd
    jitter := time.Duration(rand.Intn(300)) * time.Second
    
    for {
        select {
        case <-ticker.C:
            currentHour := time.Now().Hour()
            if currentHour == s.syncHour && s.isDirty {
                // Wait for jitter
                time.Sleep(jitter)
                s.attemptSyncWithRetry(3)
            }
        case <-s.syncTrigger:
            if s.isDirty {
                s.attemptSyncWithRetry(1)
            }
        case <-s.shutdownChan:
            return
        }
    }
}

func (s *Service) attemptSyncWithRetry(maxRetries int) {
    for attempt := 1; attempt <= maxRetries; attempt++ {
        if err := s.syncToGitHubEnhanced(); err != nil {
            if attempt == maxRetries {
                // Log final failure
                fmt.Printf("GitHub sync failed after %d attempts: %v\n", maxRetries, err)
                return
            }
            
            // Exponential backoff
            backoff := time.Duration(attempt*attempt) * time.Minute
            fmt.Printf("GitHub sync attempt %d failed, retrying in %v: %v\n", attempt, backoff, err)
            time.Sleep(backoff)
        } else {
            fmt.Printf("GitHub sync successful on attempt %d\n", attempt)
            return
        }
    }
}

func (s *Service) syncToGitHubEnhanced() error {
    s.mu.RLock()
    defer s.mu.RUnlock()
    
    ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
    defer cancel()
    
    // Get current file state
    fileContent, _, resp, err := s.githubClient.Repositories.GetContents(
        ctx, s.owner, s.repo, s.dataPath,
        &github.RepositoryContentGetOptions{Ref: s.branch},
    )
    
    var currentSHA *string
    if err != nil {
        // File might not exist yet
        if resp != nil && resp.StatusCode != 404 {
            return fmt.Errorf("failed to get current file: %w", err)
        }
    } else {
        currentSHA = fileContent.SHA
    }
    
    // Serialize current data with metadata
    tomlData, err := s.serializeToTOMLWithMetadata()
    if err != nil {
        return fmt.Errorf("failed to serialize TOML: %w", err)
    }
    
    // Create commit message with post count and summary
    postCount := len(s.socialData.Posts)
    recentPosts := s.getRecentPostsSummary(3)
    message := fmt.Sprintf("Social posts sync: %d total posts\n\nRecent posts:\n%s", 
        postCount, recentPosts)
    
    // Commit to GitHub
    opts := &github.RepositoryContentFileOptions{
        Message: &message,
        Content: tomlData,
        Branch:  &s.branch,
    }
    
    if currentSHA != nil {
        opts.SHA = currentSHA
    }
    
    _, _, err = s.githubClient.Repositories.UpdateFile(
        ctx, s.owner, s.repo, s.dataPath, opts,
    )
    if err != nil {
        return fmt.Errorf("failed to update GitHub file: %w", err)
    }
    
    // Mark as synced
    s.isDirty = false
    fmt.Printf("Successfully synced %d posts to GitHub\n", postCount)
    
    return nil
}

func (s *Service) serializeToTOMLWithMetadata() ([]byte, error) {
    // Add sync metadata
    data := *s.socialData
    data.LastSync = time.Now().UTC()
    data.PostCount = len(data.Posts)
    
    var buf strings.Builder
    buf.WriteString(fmt.Sprintf("# Social Posts Data\n"))
    buf.WriteString(fmt.Sprintf("# Last synced: %s\n", data.LastSync.Format(time.RFC3339)))
    buf.WriteString(fmt.Sprintf("# Total posts: %d\n\n", data.PostCount))
    
    encoder := toml.NewEncoder(&buf)
    if err := encoder.Encode(data); err != nil {
        return nil, err
    }
    
    return []byte(buf.String()), nil
}

func (s *Service) getRecentPostsSummary(count int) string {
    if len(s.socialData.Posts) == 0 {
        return "No posts yet"
    }
    
    // Sort by date descending (most recent first)
    posts := make([]Post, len(s.socialData.Posts))
    copy(posts, s.socialData.Posts)
    
    for i := 0; i < len(posts)-1; i++ {
        for j := i + 1; j < len(posts); j++ {
            if posts[i].Date.Before(posts[j].Date) {
                posts[i], posts[j] = posts[j], posts[i]
            }
        }
    }
    
    var summary strings.Builder
    for i := 0; i < count && i < len(posts); i++ {
        post := posts[i]
        preview := post.Content
        if len(preview) > 50 {
            preview = preview[:50] + "..."
        }
        summary.WriteString(fmt.Sprintf("- %s: %s\n", 
            post.Date.Format("Jan 2 15:04"), preview))
    }
    
    return summary.String()
}
```

### Container Rebuild Trigger
```yaml
# .github/workflows/prospero-rebuild.yml
name: Prospero Container Rebuild
on:
  push:
    paths:
      - 'hugo-site/data/social.toml'
      - 'hugo-site/content/**'
      - 'hugo-site/layouts/**'
      - 'hugo-site/static/**'

jobs:
  rebuild-container:
    runs-on: ubuntu-latest
    steps:
      - name: Checkout code
        uses: actions/checkout@v4
        
      - name: Set up Docker Buildx
        uses: docker/setup-buildx-action@v3
        
      - name: Login to Container Registry  
        uses: docker/login-action@v3
        with:
          registry: ghcr.io
          username: ${{ github.actor }}
          password: ${{ secrets.GITHUB_TOKEN }}
          
      - name: Build and push container
        uses: docker/build-push-action@v5
        with:
          context: .
          file: deploy/magic-container/Containerfile
          push: true
          tags: |
            ghcr.io/${{ github.repository }}/prospero:latest
            ghcr.io/${{ github.repository }}/prospero:${{ github.sha }}
          cache-from: type=gha
          cache-to: type=gha,mode=max
          
      - name: Deploy to Magic Containers
        env:
          BUNNY_API_KEY: ${{ secrets.BUNNY_API_KEY }}
        run: |
          # Trigger Magic Container deployment via Bunny.net API
          curl -X POST "https://api.bunny.net/deploy/magic-container" \
            -H "Authorization: Bearer $BUNNY_API_KEY" \
            -H "Content-Type: application/json" \
            -d '{
              "image": "ghcr.io/${{ github.repository }}/prospero:${{ github.sha }}",
              "environment": {
                "GITHUB_TOKEN": "${{ secrets.SOCIAL_GITHUB_TOKEN }}",
                "GITHUB_OWNER": "${{ github.repository_owner }}",
                "GITHUB_REPO": "${{ github.event.repository.name }}",
                "AGE_ENCRYPTION_PASSWORD": "${{ secrets.AGE_ENCRYPTION_PASSWORD }}"
              }
            }'
```

## Hugo Build Optimization Strategies

### 1. Incremental Build Optimization
```go
// internal/features/hugo/optimizer.go
package hugo

import (
    "crypto/md5"
    "fmt"
    "path/filepath"
    "time"
)

type BuildOptimizer struct {
    lastContentHash string
    lastBuildTime   time.Time
    cacheDir        string
}

func (b *BuildOptimizer) ShouldRebuild(contentHash string) bool {
    // Skip rebuild if content hasn't changed
    if b.lastContentHash == contentHash && 
       time.Since(b.lastBuildTime) < time.Minute {
        return false
    }
    return true
}

func (s *LiveService) optimizedBuild() error {
    // Calculate content hash
    contentHash, err := s.calculateContentHash()
    if err != nil {
        return err
    }
    
    // Check if rebuild is needed
    if !s.optimizer.ShouldRebuild(contentHash) {
        return nil // Skip rebuild
    }
    
    // Perform build
    startTime := time.Now()
    if err := s.buildSite(); err != nil {
        return err
    }
    buildDuration := time.Since(startTime)
    
    // Update optimizer state
    s.optimizer.lastContentHash = contentHash
    s.optimizer.lastBuildTime = time.Now()
    
    fmt.Printf("Hugo build completed in %v\n", buildDuration)
    return nil
}

func (s *LiveService) calculateContentHash() (string, error) {
    hasher := md5.New()
    
    // Hash data files
    dataFiles := []string{"social.toml", "config.yaml"}
    for _, file := range dataFiles {
        path := filepath.Join(s.workDir, "data", file)
        if data, err := afero.ReadFile(s.workFS, path); err == nil {
            hasher.Write(data)
        }
    }
    
    // Hash key template files
    templateFiles := []string{
        "layouts/index.html",
        "layouts/partials/social-feed.html",
    }
    for _, file := range templateFiles {
        path := filepath.Join(s.workDir, file)
        if data, err := afero.ReadFile(s.workFS, path); err == nil {
            hasher.Write(data)
        }
    }
    
    return fmt.Sprintf("%x", hasher.Sum(nil)), nil
}
```

### 2. Memory Management
```go
// Memory-efficient build pipeline
type BuildPipeline struct {
    memLimit    int64
    buildQueue  chan BuildRequest
    workers     int
}

func (s *LiveService) initBuildPipeline() {
    s.pipeline = &BuildPipeline{
        memLimit:   100 * 1024 * 1024, // 100MB limit
        buildQueue: make(chan BuildRequest, 10),
        workers:    2, // Limit concurrent builds
    }
    
    // Start build workers
    for i := 0; i < s.pipeline.workers; i++ {
        go s.buildWorker()
    }
}

func (s *LiveService) buildWorker() {
    for req := range s.pipeline.buildQueue {
        // Monitor memory usage
        if err := s.checkMemoryUsage(); err != nil {
            req.resultChan <- err
            continue
        }
        
        // Perform build
        err := s.performBuild(req)
        req.resultChan <- err
        
        // Force GC after build
        runtime.GC()
    }
}
```

### 3. Blue-Green Deployment
```go
// Zero-downtime updates with blue-green deployment
type BlueGreenService struct {
    active   *SiteVersion // Currently serving
    standby  *SiteVersion // Building new version
    mu       sync.RWMutex
}

type SiteVersion struct {
    version   string
    files     fs.FS
    buildTime time.Time
}

func (bg *BlueGreenService) UpdateSite(newFiles fs.FS) {
    bg.mu.Lock()
    defer bg.mu.Unlock()
    
    // Update standby version
    bg.standby = &SiteVersion{
        version:   fmt.Sprintf("v%d", time.Now().Unix()),
        files:     newFiles,
        buildTime: time.Now(),
    }
    
    // Atomic swap
    bg.active, bg.standby = bg.standby, bg.active
    
    fmt.Printf("Site updated to version %s\n", bg.active.version)
}

func (bg *BlueGreenService) ServeHTTP(w http.ResponseWriter, r *http.Request) {
    bg.mu.RLock()
    site := bg.active
    bg.mu.RUnlock()
    
    if site == nil || site.files == nil {
        http.Error(w, "Site not ready", http.StatusServiceUnavailable)
        return
    }
    
    // Add version header for debugging
    w.Header().Set("X-Site-Version", site.version)
    http.FileServer(http.FS(site.files)).ServeHTTP(w, r)
}
```

## Complete Development Workflow

### 1. Project Structure Setup
```
prospero/
├── hugo-site/                  # Hugo source (not embedded)
│   ├── content/
│   │   ├── posts/             # Blog posts (manual)
│   │   └── pages/             # Static pages
│   ├── data/
│   │   └── social.toml        # Social posts (API managed)
│   ├── layouts/
│   │   ├── index.html
│   │   ├── _default/
│   │   └── partials/
│   │       └── social-feed.html
│   ├── static/
│   │   ├── css/
│   │   ├── js/
│   │   └── favicon.ico        # No images here (use Bunny Storage)
│   └── hugo.toml
├── internal/
│   ├── features/
│   │   ├── hugo/              # NEW: Hugo integration
│   │   │   ├── service.go     # Static embed service
│   │   │   ├── live.go        # In-memory live service
│   │   │   └── static/        # Built site (embedded)
│   │   ├── social/            # NEW: Social posts
│   │   │   ├── service.go
│   │   │   └── github.go
│   │   ├── topten/            # Existing
│   │   └── shakespert/        # Existing
│   └── web/
│       └── handlers/
│           └── social.go      # NEW: Social API handlers
└── justfile                   # Updated with Hugo commands
```

### 2. Enhanced Justfile Commands
```makefile
# Hugo-related commands
build-hugo:
    #!/usr/bin/env bash
    cd hugo-site
    hugo --minify --destination ../internal/features/hugo/static

build-hugo-dev:
    #!/usr/bin/env bash
    cd hugo-site
    hugo --buildDrafts --destination ../internal/features/hugo/static

dev-hugo:
    #!/usr/bin/env bash
    cd hugo-site
    hugo server --buildDrafts --port 1313 --bind 0.0.0.0

# Build everything
build: build-hugo
    go build -o bin/prospero ./cmd/prospero

build-dev: build-hugo-dev
    go build -o bin/prospero ./cmd/prospero

# Development servers
dev-full:
    #!/usr/bin/env bash
    echo "Starting Hugo dev server on :1313"
    cd hugo-site && hugo server --buildDrafts --port 1313 --bind 0.0.0.0 &
    HUGO_PID=$!
    
    echo "Starting prospero API server on :8080"
    sleep 2
    ./bin/prospero serve --http-port 8080 --hugo-dev-mode &
    API_PID=$!
    
    echo "Servers started. Access:"
    echo "  Hugo site: http://localhost:1313"
    echo "  API + embedded site: http://localhost:8080"
    echo ""
    echo "Press Ctrl+C to stop both servers"
    
    trap 'kill $HUGO_PID $API_PID' INT
    wait

# Social posts commands
social-post content:
    curl -X POST http://localhost:8080/api/social/posts \
      -H "Content-Type: application/json" \
      -d '{"content":"{{content}}"}'

social-posts:
    curl -s http://localhost:8080/api/social/posts | jq .

social-sync:
    curl -X POST http://localhost:8080/api/social/sync

# Container commands (updated)
container-magic: build
    podman build -f deploy/magic-container/Containerfile -t localhost/prospero:latest .

container-run-magic:
    podman run -p 8080:8080 -p 2222:2222 \
      -e GITHUB_TOKEN=${GITHUB_TOKEN} \
      -e GITHUB_OWNER=${GITHUB_OWNER:-$(git config user.name)} \
      -e GITHUB_REPO=prospero \
      -e AGE_ENCRYPTION_PASSWORD=${AGE_ENCRYPTION_PASSWORD} \
      localhost/prospero:latest

# Testing social integration
test-social:
    #!/usr/bin/env bash
    echo "Testing social posts integration..."
    
    # Start server in background
    ./bin/prospero serve --http-port 8080 &
    SERVER_PID=$!
    sleep 3
    
    # Test post creation
    echo "Creating test post..."
    curl -X POST http://localhost:8080/api/social/posts \
      -H "Content-Type: application/json" \
      -d '{"content":"Test post from automated test #testing"}' | jq .
    
    # Test post retrieval
    echo "Retrieving posts..."
    curl -s http://localhost:8080/api/social/posts?limit=5 | jq .
    
    # Test site serving
    echo "Testing site serving..."
    curl -s -o /dev/null -w "%{http_code}" http://localhost:8080/
    
    # Cleanup
    kill $SERVER_PID
    echo "Tests completed"
```

### 3. Local Development Setup
```bash
# Initial setup
git clone https://github.com/username/prospero.git
cd prospero

# Install Hugo (macOS)
brew install hugo

# Install dependencies
go mod download

# Set up environment
cp .env.example .env
# Edit .env with your GitHub token and other secrets

# First build
just build

# Start development servers
just dev-full
```

### 4. Development Modes

#### Mode 1: Pure Hugo Development
```bash
# Hugo only (fastest iteration)
just dev-hugo
# Site available at http://localhost:1313
# Hot reload on file changes
# Good for: template changes, content updates, CSS/JS work
```

#### Mode 2: API Development  
```bash
# Prospero with embedded Hugo
just build && ./bin/prospero serve --http-port 8080
# Site + APIs available at http://localhost:8080
# Good for: API development, testing social posts
```

#### Mode 3: Full Development
```bash
# Both servers running
just dev-full
# Hugo: http://localhost:1313 (live reload)
# APIs: http://localhost:8080 (embedded site)
# Good for: full-stack development
```

### 5. Testing Strategy

#### Unit Tests
```go
// internal/features/social/service_test.go
package social_test

import (
    "testing"
    "prospero/internal/features/social"
    "maragu.dev/is"
)

func TestService_CreatePost(t *testing.T) {
    tests := []struct {
        name    string
        content string
        wantErr bool
    }{
        {
            name:    "should create post with simple content",
            content: "Hello world!",
            wantErr: false,
        },
        {
            name:    "should extract hashtags from content",
            content: "Testing #golang #hugo integration",
            wantErr: false,
        },
        {
            name:    "should reject empty content",
            content: "",
            wantErr: true,
        },
    }

    for _, test := range tests {
        t.Run(test.name, func(t *testing.T) {
            service := setupTestService(t)
            
            post, err := service.CreatePost(test.content)
            
            if test.wantErr {
                is.Error(t, err)
                is.Nil(t, post)
            } else {
                is.NotError(t, err)
                is.NotNil(t, post)
                is.Equal(t, test.content, post.Content)
                
                if strings.Contains(test.content, "#") {
                    is.True(t, len(post.Tags) > 0)
                }
            }
        })
    }
}

func setupTestService(t *testing.T) *social.Service {
    t.Helper()
    
    // Mock Hugo service for testing
    mockHugo := &MockHugoService{}
    
    cfg := social.Config{
        GitHubToken:  "test-token",
        GitHubOwner:  "test-owner",  
        GitHubRepo:   "test-repo",
        GitHubBranch: "main",
        DataPath:     "hugo-site/data/social.toml",
        SyncEnabled:  false, // Disable for tests
    }
    
    service, err := social.NewService(mockHugo, cfg)
    is.NotError(t, err)
    
    return service
}
```

#### Integration Tests
```bash
# Test script for full integration
#!/usr/bin/env bash
# test-integration.sh

set -e

echo "Starting integration tests..."

# Build and start server
just build
./bin/prospero serve --http-port 8080 &
SERVER_PID=$!

# Wait for startup
sleep 5

# Test health endpoint
curl -f http://localhost:8080/health

# Test static site serving
curl -f http://localhost:8080/ | grep -q "<html"

# Test API endpoints
curl -f http://localhost:8080/api/topten
curl -f http://localhost:8080/api/shakespert/works

# Test social posts API
POST_RESPONSE=$(curl -X POST http://localhost:8080/api/social/posts \
  -H "Content-Type: application/json" \
  -d '{"content":"Integration test post #testing"}')

echo $POST_RESPONSE | jq -e '.success == true'

# Test posts retrieval
curl -f http://localhost:8080/api/social/posts | jq -e '.posts | length >= 1'

# Cleanup
kill $SERVER_PID

echo "All integration tests passed!"
```

### 6. Container Development
```dockerfile
# deploy/magic-container/Containerfile (updated)
FROM alpine:3.19 AS hugo-builder
RUN apk add --no-cache hugo git
WORKDIR /src
COPY hugo-site/ .
RUN hugo --minify --baseURL "${HUGO_BASE_URL:-https://prospero.example.com}"

FROM golang:1.25-alpine AS go-builder
WORKDIR /build
COPY go.mod go.sum ./
RUN go mod download
COPY . .
COPY --from=hugo-builder /src/public/ ./internal/features/hugo/static/
RUN CGO_ENABLED=0 GOOS=linux go build -o prospero ./cmd/prospero

FROM alpine:3.19
RUN apk add --no-cache ca-certificates hugo
COPY --from=go-builder /build/prospero /usr/local/bin/
WORKDIR /app
EXPOSE 8080 2222
ENTRYPOINT ["/usr/local/bin/prospero"]
CMD ["serve"]
```

### 7. Production Deployment Pipeline

#### Environment Variables
```env
# Production environment
HUGO_BASE_URL=https://your-domain.com
IMAGE_CDN_URL=https://cdn.your-domain.b-cdn.net
GITHUB_TOKEN=ghp_xxx...
GITHUB_OWNER=yourusername
GITHUB_REPO=prospero
GITHUB_BRANCH=main
AGE_ENCRYPTION_PASSWORD=xxx...
SOCIAL_SYNC_HOUR=3
```

#### Deployment Workflow
```yaml
# .github/workflows/deploy.yml
name: Deploy Prospero
on:
  push:
    branches: [main]
  workflow_dispatch:

jobs:
  test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version: '1.25'
      - name: Install Hugo
        run: |
          wget https://github.com/gohugoio/hugo/releases/download/v0.121.0/hugo_extended_0.121.0_linux-amd64.tar.gz
          tar -xzf hugo_extended_0.121.0_linux-amd64.tar.gz
          sudo mv hugo /usr/local/bin/
      - name: Run tests
        run: |
          go test ./...
          ./test-integration.sh
  
  deploy:
    needs: test
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - name: Build and deploy container
        env:
          BUNNY_API_KEY: ${{ secrets.BUNNY_API_KEY }}
        run: |
          # Build container with Hugo integration
          podman build -f deploy/magic-container/Containerfile \
            --build-arg HUGO_BASE_URL=https://your-domain.com \
            -t ghcr.io/${{ github.repository }}:${{ github.sha }} .
          
          # Push to registry
          podman push ghcr.io/${{ github.repository }}:${{ github.sha }}
          
          # Deploy to Magic Containers
          curl -X POST "https://api.bunny.net/deploy/magic-container" \
            -H "Authorization: Bearer $BUNNY_API_KEY" \
            -d '{
              "image": "ghcr.io/${{ github.repository }}:${{ github.sha }}",
              "environment": {
                "GITHUB_TOKEN": "${{ secrets.GITHUB_TOKEN }}",
                "GITHUB_OWNER": "${{ github.repository_owner }}",
                "GITHUB_REPO": "${{ github.event.repository.name }}",
                "AGE_ENCRYPTION_PASSWORD": "${{ secrets.AGE_ENCRYPTION_PASSWORD }}"
              }
            }'
```

## Decision Matrix and Implementation Guidance

### When to Use Each Approach

#### 1. Static Embed (Original Plan)
```
Choose when:
✅ Content changes infrequently (monthly or less)
✅ You want maximum simplicity  
✅ SEO is critical for all content
✅ You don't need dynamic posting
✅ Small site (< 100 pages)

Example use cases:
- Marketing site
- Documentation
- Portfolio
- Company blog with infrequent posts
```

#### 2. In-Memory Hugo (Recommended for Social Posts)  
```
Choose when:
✅ You want social media-style posting
✅ Content changes frequently (daily/hourly)
✅ You still want static site benefits
✅ You can tolerate eventual consistency
✅ You want operational simplicity

Example use cases:
- Personal site with microblog
- Developer blog with quick notes
- Project updates and announcements
- Status page with incident updates
```

#### 3. Separated Deployment
```
Choose when:
✅ You have a large, content-heavy site
✅ You have a dedicated content team
✅ You need CDN performance globally
✅ Frontend/backend developed by different teams
✅ You need independent scaling

Example use cases:
- Large corporate site
- E-commerce with heavy content
- Multi-language sites
- High-traffic blogs
```

### Implementation Phases

#### Phase 1: Basic Hugo Integration (Week 1)
```
1. Set up hugo-site/ directory structure
2. Create basic layouts and content
3. Implement static embed service (internal/features/hugo/)
4. Update HTTP routing to serve embedded files
5. Add build commands to justfile
6. Test local development workflow

Deliverable: Static Hugo site embedded in prospero
```

#### Phase 2: Social Posts Foundation (Week 2)
```
1. Design TOML schema for social posts
2. Create Hugo templates for social feed
3. Implement basic social service (in-memory only)
4. Add HTTP handlers for social API
5. Create social post management UI (optional)
6. Test post creation and display

Deliverable: Social posts working with Hugo rebuild
```

#### Phase 3: Live Hugo Service (Week 3)  
```
1. Implement in-memory Hugo live service
2. Add file system management with afero
3. Create build optimization and caching
4. Implement blue-green deployment pattern
5. Add proper error handling and recovery
6. Performance testing and tuning

Deliverable: Real-time post updates without container rebuild
```

#### Phase 4: GitHub Integration (Week 4)
```
1. Implement GitHub API client
2. Add sync scheduler with retry logic
3. Create container rebuild triggers
4. Set up GitHub Actions workflow
5. Add monitoring and alerting
6. Production deployment and testing

Deliverable: Full production-ready system with persistence
```

### Migration Strategies

#### From Current Prospero to Hugo Integration
```bash
# 1. Add Hugo alongside existing features
mkdir hugo-site
# ... set up Hugo site structure

# 2. Build side-by-side  
just build-hugo  # Creates embedded files
just build       # Builds Go with embedded Hugo

# 3. Update routing gradually
# Keep existing /api/* routes
# Add /* → Hugo for everything else

# 4. Deploy and test
# Existing features continue working
# Hugo site now available at root
```

#### From Static to In-Memory Hugo  
```bash
# 1. Start with static embed working
# 2. Add live service alongside static
# 3. Feature flag to choose which to use
# 4. Migrate gradually with fallback
# 5. Remove static service once confident
```

### Performance Benchmarks

#### Expected Performance Characteristics
```
Static Embed:
- Container size: ~35-50MB
- Memory usage: ~30-50MB
- Cold start: ~2-3 seconds
- Request latency: ~5-10ms

In-Memory Hugo:
- Container size: ~60-80MB  
- Memory usage: ~100-150MB
- Cold start: ~5-10 seconds
- Request latency: ~5-10ms (cached), ~100-500ms (rebuild)
- Hugo rebuild: ~1-5 seconds
```

#### Scaling Characteristics
```
Posts per hour: 100+ (no GitHub rate limit issues)
Concurrent rebuilds: 2 (limited by memory)
Container restarts: Graceful with embedded fallback
GitHub sync: Once per day, 5000 API calls/hour limit
```

This comprehensive plan gives you everything needed to implement Hugo integration with prospero. You can start with any phase and have a complete reference for all the technical details when working with Claude Code later.