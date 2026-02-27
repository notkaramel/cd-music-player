#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <gst/gst.h>
#include <gst/app/gstappsrc.h>
#include <cdio/cdio.h>
#include <cdio/cd_types.h>
#include <cdio/paranoia/cdda.h>

#define SECTOR_SIZE 2352

static GstElement *pipeline, *appsrc;
static CdIo_t *cdio = NULL;
static track_t first_track, last_track;
static lsn_t start_lsn, end_lsn;
static lsn_t current_lsn;

static gboolean push_data(GstElement *appsrc) {
    if (current_lsn >= end_lsn)
        return FALSE;

    GstBuffer *buffer = gst_buffer_new_allocate(NULL, SECTOR_SIZE, NULL);
    GstMapInfo map;

    gst_buffer_map(buffer, &map, GST_MAP_WRITE);

    if (cdio_read_audio_sector(cdio, map.data, current_lsn) != DRIVER_OP_SUCCESS) {
        gst_buffer_unmap(buffer, &map);
        gst_buffer_unref(buffer);
        return FALSE;
    }

    gst_buffer_unmap(buffer, &map);

    GST_BUFFER_DURATION(buffer) =
        gst_util_uint64_scale(1, GST_SECOND, 75); // 75 sectors/sec

    gst_app_src_push_buffer(GST_APP_SRC(appsrc), buffer);

    current_lsn++;
    return TRUE;
}

static gboolean feed_data(GstElement *src, guint size, gpointer user_data) {
    return push_data(src);
}

int main(int argc, char *argv[]) {
    gst_init(&argc, &argv);

    // Open CD device
    cdio = cdio_open("/dev/cdrom", DRIVER_UNKNOWN);
    if (!cdio) {
        fprintf(stderr, "Failed to open CD device\n");
        return 1;
    }

    first_track = cdio_get_first_track_num(cdio);
    last_track = cdio_get_last_track_num(cdio);

    printf("Tracks: %d - %d\n", first_track, last_track);

    // Play first track
    start_lsn = cdio_get_track_lsn(cdio, first_track);
    end_lsn   = cdio_get_track_last_lsn(cdio, first_track);
    current_lsn = start_lsn;

    // Build GStreamer pipeline
    pipeline = gst_pipeline_new("cd-player");

    appsrc = gst_element_factory_make("appsrc", "source");
    GstElement *convert = gst_element_factory_make("audioconvert", NULL);
    GstElement *resample = gst_element_factory_make("audioresample", NULL);
    GstElement *sink = gst_element_factory_make("autoaudiosink", NULL);

    gst_bin_add_many(GST_BIN(pipeline),
                     appsrc, convert, resample, sink, NULL);

    gst_element_link_many(appsrc, convert, resample, sink, NULL);

    // Set audio format: CDDA is 44.1kHz, 16-bit, stereo PCM
    GstCaps *caps = gst_caps_new_simple(
        "audio/x-raw",
        "format", G_TYPE_STRING, "S16LE",
        "layout", G_TYPE_STRING, "interleaved",
        "rate", G_TYPE_INT, 44100,
        "channels", G_TYPE_INT, 2,
        NULL);

    g_object_set(appsrc,
                 "caps", caps,
                 "format", GST_FORMAT_TIME,
                 NULL);

    gst_caps_unref(caps);

    g_signal_connect(appsrc, "need-data", G_CALLBACK(feed_data), NULL);

    gst_element_set_state(pipeline, GST_STATE_PLAYING);

    printf("Playing track %d...\n", first_track);

    // Run main loop
    GMainLoop *loop = g_main_loop_new(NULL, FALSE);
    g_main_loop_run(loop);

    gst_element_set_state(pipeline, GST_STATE_NULL);
    gst_object_unref(pipeline);
    cdio_destroy(cdio);

    return 0;
}