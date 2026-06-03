import { useCallback, useState } from 'react'
import { motion, AnimatePresence } from 'framer-motion'
import { API, STREAM_URL } from '../config'
import OverlayCanvas from './OverlayCanvas'

export default function VideoExperience({ overlay, heatmapCells, onUploadStart, onUploadComplete }) {
  const [dragging, setDragging] = useState(false)
  const [uploading, setUploading] = useState(false)
  const [progress, setProgress] = useState(0)
  const [streamKey, setStreamKey] = useState(0)
  const [hasVideo, setHasVideo] = useState(false)
  const [uploadError, setUploadError] = useState('')

  const uploadFile = useCallback(async (file) => {
    if (!file) return
    const ext = file.name.split('.').pop()?.toLowerCase()
    if (!['mp4', 'avi', 'mov', 'mkv', 'webm'].includes(ext)) {
      setUploadError('Unsupported file type. Choose MP4, AVI, MOV, MKV, or WebM footage.')
      return
    }

    setUploading(true)
    setHasVideo(false)
    onUploadStart?.()
    setUploadError('')
    setProgress(0)

    const form = new FormData()
    form.append('video', file)

    const xhr = new XMLHttpRequest()
    xhr.upload.onprogress = (e) => {
      if (e.lengthComputable) setProgress(Math.round((e.loaded / e.total) * 100))
    }
    xhr.onload = () => {
      setUploading(false)
      if (xhr.status >= 200 && xhr.status < 300) {
        setProgress(100)
        setHasVideo(true)
        setStreamKey((k) => k + 1)
        onUploadComplete?.()
        return
      }
      setUploadError('Upload failed. Check that the detection service is running and try again.')
    }
    xhr.onerror = () => {
      setUploading(false)
      setUploadError('Upload could not reach the API gateway. Check that the system is running.')
    }
    xhr.open('POST', `${API}/video/upload`)
    xhr.send(form)
  }, [onUploadStart, onUploadComplete])

  const onDrop = (e) => {
    e.preventDefault()
    setDragging(false)
    uploadFile(e.dataTransfer.files[0])
  }

  const onFileChange = (e) => {
    const file = e.target.files[0]
    e.target.value = ''
    uploadFile(file)
  }

  return (
    <div className="relative w-full h-full flex items-center justify-center bg-void">
      {/* Video stream */}
      <div className="relative w-full h-full max-h-full">
        {hasVideo && (
          <>
            <img
              key={streamKey}
              src={`${STREAM_URL}?t=${streamKey}`}
              alt="Live AI Feed"
              id="live-ai-feed"
              className="w-full h-full object-contain"
            />
            <OverlayCanvas
              detections={overlay.detections}
              frameSize={overlay.frame_size}
              zones={overlay.zones}
              heatmapCells={heatmapCells}
              videoElementId="live-ai-feed"
            />
          </>
        )}

        {/* Live badge */}
        {hasVideo && (
          <div className="absolute top-4 left-4 glass px-3 py-1.5 rounded-full flex items-center gap-2 text-xs font-mono">
            <span className="w-2 h-2 rounded-full bg-white animate-pulse" />
            AI LIVE
          </div>
        )}

        {/* Upload zone overlay when no video */}
        <AnimatePresence>
          {!hasVideo && !uploading && (
            <motion.div
              initial={{ opacity: 0 }}
              animate={{ opacity: 1 }}
              exit={{ opacity: 0 }}
              className={`absolute inset-0 flex items-center justify-center m-8 rounded-2xl border-2 border-dashed transition-colors ${
                dragging ? 'border-white bg-white/10' : 'border-white/20 bg-black/40'
              }`}
              onDragOver={(e) => { e.preventDefault(); setDragging(true) }}
              onDragLeave={() => setDragging(false)}
              onDrop={onDrop}
            >
              <div className="text-center pointer-events-auto">
                <p className="text-2xl font-light tracking-wide text-white/90 mb-2">
                  Drop CCTV footage here
                </p>
                <p className="text-sm text-white/40 mb-6">MP4 / AVI / MOV / MKV / WEBM</p>
                <label className="glass px-6 py-3 rounded-full cursor-pointer hover:bg-white/10 transition text-sm">
                  Browse files
                  <input
                    type="file"
                    accept=".mp4,.avi,.mov,.mkv,.webm"
                    className="hidden"
                    onChange={onFileChange}
                  />
                </label>
              </div>
            </motion.div>
          )}
        </AnimatePresence>

        {/* Upload progress */}
        {uploading && (
          <div className="absolute bottom-8 left-1/2 -translate-x-1/2 w-64 glass rounded-full px-4 py-3">
            <p className="text-xs text-white/60 mb-2 text-center">Uploading and starting inference...</p>
            <div className="h-1 bg-white/10 rounded-full overflow-hidden">
              <motion.div
                className="h-full bg-white"
                initial={{ width: 0 }}
                animate={{ width: `${progress}%` }}
              />
            </div>
          </div>
        )}

        {uploadError && (
          <div className="absolute bottom-8 left-1/2 -translate-x-1/2 max-w-md glass rounded-xl px-4 py-3 text-center text-xs text-red-200">
            {uploadError}
          </div>
        )}

        {/* Mini upload button when video active */}
        {hasVideo && (
          <label className="absolute top-4 right-4 glass px-4 py-2 rounded-full cursor-pointer text-xs hover:bg-white/10 transition pointer-events-auto">
            + New footage
            <input
              type="file"
              accept=".mp4,.avi,.mov,.mkv,.webm"
              className="hidden"
              onChange={onFileChange}
            />
          </label>
        )}
      </div>
    </div>
  )
}
