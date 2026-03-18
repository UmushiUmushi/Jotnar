package com.jotnar.network

import com.jotnar.network.models.*
import okhttp3.MultipartBody
import okhttp3.RequestBody
import retrofit2.Response
import retrofit2.http.*

interface JotnarApi {

    // Auth (public)

    @POST("auth/pair")
    suspend fun pair(@Body request: PairRequest): Response<PairResponse>

    @POST("auth/recover")
    suspend fun recover(@Body request: RecoverRequest): Response<RecoverResponse>

    @GET("status")
    suspend fun getStatus(): Response<StatusResponse>

    // Auth (authenticated)

    @POST("auth/pair/new")
    suspend fun generatePairingCode(): Response<PairingCodeResponse>

    // Capture

    @Multipart
    @POST("capture")
    suspend fun capture(
        @Part screenshot: MultipartBody.Part,
        @Part("captured_at") capturedAt: RequestBody? = null
    ): Response<CaptureResponse>

    @Multipart
    @POST("capture/batch")
    suspend fun batchCapture(
        @Part screenshots: List<MultipartBody.Part>,
        @Part("captured_at") capturedAt: @JvmSuppressWildcards List<RequestBody>? = null
    ): Response<BatchCaptureResponse>

    // Journal

    @GET("journal")
    suspend fun listJournal(
        @Query("limit") limit: Int = 20,
        @Query("offset") offset: Int = 0
    ): Response<JournalListResponse>

    @GET("journal/{id}")
    suspend fun getJournalEntry(@Path("id") id: String): Response<JournalEntryResponse>

    @PUT("journal/{id}")
    suspend fun updateJournalEntry(
        @Path("id") id: String,
        @Body request: UpdateJournalRequest
    ): Response<JournalEntryResponse>

    @DELETE("journal/{id}")
    suspend fun deleteJournalEntry(@Path("id") id: String): Response<DeleteResponse>

    // Metadata + Reconsolidation

    @GET("journal/{id}/metadata")
    suspend fun getEntryMetadata(@Path("id") id: String): Response<MetadataListResponse>

    @POST("journal/{id}/preview")
    suspend fun previewReconsolidation(
        @Path("id") id: String,
        @Body request: ReconsolidateRequest
    ): Response<PreviewResponse>

    @POST("journal/{id}/reconsolidate")
    suspend fun commitReconsolidation(
        @Path("id") id: String,
        @Body request: ReconsolidateRequest
    ): Response<JournalEntryResponse>

    // Config

    @GET("config")
    suspend fun getConfig(): Response<ConfigResponse>

    @PUT("config")
    suspend fun updateConfig(@Body request: UpdateConfigRequest): Response<ConfigResponse>
}
